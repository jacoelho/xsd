package schemaxml

import (
	"errors"
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/state"
	"github.com/jacoelho/xsd/internal/xmllex"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Parse builds the minimal DOM used by the validator from XML input.
func Parse(r io.Reader) (*Document, error) {
	doc := &Document{root: InvalidNode}
	if err := ParseIntoWithOptions(r, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// ParseIntoWithOptions builds the minimal DOM into an existing document with reader options.
func ParseIntoWithOptions(r io.Reader, doc *Document, opts ...xmlstream.Option) (err error) {
	if doc == nil {
		return fmt.Errorf("nil XML document")
	}

	doc.reset()
	defer func() {
		if err != nil {
			doc.reset()
		}
	}()

	decoder, err := xmlstream.NewReader(r, opts...)
	if err != nil {
		return fmt.Errorf("xml reader: %w", err)
	}

	return parseDOM(decoder, doc, parseModeDocument, namespaceAttrsScopeOnly, xmlstream.Event{})
}

// ParseSubtreeInto builds a document from a subtree rooted at start.
func ParseSubtreeInto(reader *xmlstream.Reader, start xmlstream.Event, doc *Document) (err error) {
	if doc == nil {
		return fmt.Errorf("nil XML document")
	}

	doc.reset()
	defer func() {
		if err != nil {
			doc.reset()
		}
	}()
	if reader == nil {
		return fmt.Errorf("nil XML reader")
	}
	if start.Kind != xmlstream.EventStartElement {
		return fmt.Errorf("expected start element event")
	}

	return parseDOM(reader, doc, parseModeSubtree, namespaceAttrsSubtreeRootInScope, start)
}

type parseMode uint8

const (
	parseModeDocument parseMode = iota
	parseModeSubtree
)

type namespaceAttrsMode uint8

const (
	namespaceAttrsScopeOnly namespaceAttrsMode = iota
	namespaceAttrsSubtreeRootInScope
)

type domBuilder struct {
	doc             *Document
	reader          *xmlstream.Reader
	nodeStack       state.StateStack[NodeID]
	childCountStack state.StateStack[int]
	attrsScratch    []Attr
	namespaceMode   namespaceAttrsMode
}

func newDOMBuilder(doc *Document, reader *xmlstream.Reader, namespaceMode namespaceAttrsMode) *domBuilder {
	return &domBuilder{
		doc:             doc,
		reader:          reader,
		namespaceMode:   namespaceMode,
		nodeStack:       state.NewStateStack[NodeID](16),
		childCountStack: state.NewStateStack[int](16),
	}
}

func (b *domBuilder) hasOpenNode() bool {
	return b != nil && b.nodeStack.Len() > 0
}

func (b *domBuilder) addStart(event xmlstream.Event) {
	if b == nil {
		return
	}

	parent := InvalidNode
	if p, ok := b.nodeStack.Peek(); ok {
		parent = p
	}
	if parent != InvalidNode {
		count, _ := b.childCountStack.Pop()
		b.childCountStack.Push(count + 1)
	}

	b.attrsScratch = b.attrsScratch[:0]
	for _, attr := range event.Attrs {
		b.attrsScratch = append(b.attrsScratch, Attr{
			namespace: attr.Name.Namespace,
			local:     attr.Name.Local,
			value:     string(attr.Value),
		})
	}
	b.attrsScratch = b.appendNamespaceAttrs(b.attrsScratch, event.ScopeDepth)

	id := b.doc.addNode(event.Name.Namespace, event.Name.Local, b.attrsScratch, parent)
	if parent == InvalidNode {
		b.doc.root = id
	}
	b.nodeStack.Push(id)
	b.childCountStack.Push(0)
}

func (b *domBuilder) addEnd() bool {
	if b == nil {
		return false
	}
	if _, ok := b.nodeStack.Pop(); ok {
		_, _ = b.childCountStack.Pop()
		return b.nodeStack.Len() == 0 && b.doc.root != InvalidNode
	}
	return false
}

func (b *domBuilder) addCharData(text []byte) {
	if b == nil {
		return
	}
	nodeID, ok := b.nodeStack.Peek()
	if !ok {
		return
	}
	node := &b.doc.nodes[nodeID]
	textOff := len(node.text)
	node.text = append(node.text, text...)
	count, _ := b.childCountStack.Peek()
	b.doc.addTextSegment(nodeID, count, textOff, len(text))
}

func (b *domBuilder) appendNamespaceAttrs(attrs []Attr, scopeDepth int) []Attr {
	if b == nil {
		return attrs
	}
	isRoot := b.nodeStack.Len() == 0
	if b.namespaceMode == namespaceAttrsSubtreeRootInScope && isRoot {
		return appendInScopeNamespaceAttrs(attrs, b.reader, scopeDepth)
	}
	return appendScopeNamespaceAttrs(attrs, b.reader, scopeDepth)
}

func parseDOM(reader *xmlstream.Reader, doc *Document, mode parseMode, namespaceMode namespaceAttrsMode, start xmlstream.Event) error {
	builder := newDOMBuilder(doc, reader, namespaceMode)
	docState := xmllex.NewDocumentState()
	depth := 0

	if mode == parseModeSubtree {
		builder.addStart(start)
		depth = 1
		docState.OnStartElement()
	}

	for mode != parseModeSubtree || depth != 0 {

		event, err := reader.Next()
		if mode == parseModeDocument && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("xml read: %w", err)
		}

		switch event.Kind {
		case xmlstream.EventStartElement:
			if mode == parseModeDocument && !docState.StartElementAllowed() {
				return fmt.Errorf("unexpected element %s after document end", event.Name.Local)
			}
			builder.addStart(event)
			if mode == parseModeSubtree {
				depth++
			}
			docState.OnStartElement()

		case xmlstream.EventEndElement:
			closeRoot := builder.addEnd() && mode == parseModeDocument
			docState.OnEndElement(closeRoot)
			if mode == parseModeSubtree {
				depth--
			}

		case xmlstream.EventCharData:
			if !builder.hasOpenNode() {
				if mode == parseModeSubtree {
					continue
				}
				if !docState.ValidateOutsideCharData(event.Text) {
					return fmt.Errorf("unexpected character data outside root element")
				}
				continue
			}
			builder.addCharData(event.Text)

		case xmlstream.EventComment, xmlstream.EventPI, xmlstream.EventDirective:
			if mode == parseModeDocument && !builder.hasOpenNode() {
				docState.OnOutsideMarkup()
			}
		}
	}

	if doc.root == InvalidNode {
		return io.ErrUnexpectedEOF
	}

	doc.buildChildren()
	doc.buildTextSegments()
	return nil
}

func appendInScopeNamespaceAttrs(attrs []Attr, reader *xmlstream.Reader, scopeDepth int) []Attr {
	if reader == nil || scopeDepth < 0 {
		return attrs
	}

	prefixOrder := make([]string, 0, 8)
	prefixURI := make(map[string]string, 8)
	for depth := 0; depth <= scopeDepth; depth++ {
		for decl := range reader.NamespaceDeclsSeq(depth) {
			if _, exists := prefixURI[decl.Prefix]; !exists {
				prefixOrder = append(prefixOrder, decl.Prefix)
			}
			prefixURI[decl.Prefix] = decl.URI
		}
	}

	for _, prefix := range prefixOrder {
		local := prefix
		if local == "" {
			local = "xmlns"
		}
		attrs = append(attrs, Attr{
			namespace: XMLNSNamespace,
			local:     local,
			value:     prefixURI[prefix],
		})
	}
	return attrs
}

func appendScopeNamespaceAttrs(attrs []Attr, reader *xmlstream.Reader, scopeDepth int) []Attr {
	if reader == nil || scopeDepth < 0 {
		return attrs
	}
	for decl := range reader.NamespaceDeclsSeq(scopeDepth) {
		local := decl.Prefix
		if local == "" {
			local = "xmlns"
		}
		attrs = append(attrs, Attr{
			namespace: XMLNSNamespace,
			local:     local,
			value:     decl.URI,
		})
	}
	return attrs
}
