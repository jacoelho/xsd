package schemaast

import (
	"errors"
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

// Parse builds the minimal DOM used by the validator from XML input.
func parseDocument(r io.Reader) (*Document, error) {
	doc := &Document{root: InvalidNode}
	if err := parseIntoWithOptions(r, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// parseIntoWithOptions builds the minimal DOM into an existing document with reader options.
func parseIntoWithOptions(r io.Reader, doc *Document, opts ...xmlstream.Option) (err error) {
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

// parseSubtreeInto builds a document from a subtree rooted at start.
func parseSubtreeInto(reader *xmlstream.Reader, start xmlstream.Event, doc *Document) (err error) {
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
	nodeStack       []NodeID
	childCountStack []int
	attrsScratch    []Attr
	namespaceMode   namespaceAttrsMode
}

func newDOMBuilder(doc *Document, reader *xmlstream.Reader, namespaceMode namespaceAttrsMode) *domBuilder {
	return &domBuilder{
		doc:             doc,
		reader:          reader,
		namespaceMode:   namespaceMode,
		nodeStack:       make([]NodeID, 0, 16),
		childCountStack: make([]int, 0, 16),
	}
}

func (b *domBuilder) hasOpenNode() bool {
	return b != nil && len(b.nodeStack) > 0
}

func (b *domBuilder) addStart(event xmlstream.Event) {
	if b == nil {
		return
	}

	parent := InvalidNode
	if len(b.nodeStack) > 0 {
		parent = b.nodeStack[len(b.nodeStack)-1]
	}
	if parent != InvalidNode {
		last := len(b.childCountStack) - 1
		b.childCountStack[last]++
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
	b.nodeStack = append(b.nodeStack, id)
	b.childCountStack = append(b.childCountStack, 0)
}

func (b *domBuilder) addEnd() bool {
	if b == nil {
		return false
	}
	if len(b.nodeStack) > 0 {
		b.nodeStack = b.nodeStack[:len(b.nodeStack)-1]
		b.childCountStack = b.childCountStack[:len(b.childCountStack)-1]
		return len(b.nodeStack) == 0 && b.doc.root != InvalidNode
	}
	return false
}

func (b *domBuilder) addCharData(text []byte) {
	if b == nil {
		return
	}
	if len(b.nodeStack) == 0 {
		return
	}
	nodeID := b.nodeStack[len(b.nodeStack)-1]
	node := &b.doc.nodes[nodeID]
	textOff := len(node.text)
	node.text = append(node.text, text...)
	count := b.childCountStack[len(b.childCountStack)-1]
	b.doc.addTextSegment(nodeID, count, textOff, len(text))
}

func (b *domBuilder) appendNamespaceAttrs(attrs []Attr, scopeDepth int) []Attr {
	if b == nil {
		return attrs
	}
	isRoot := len(b.nodeStack) == 0
	if b.namespaceMode == namespaceAttrsSubtreeRootInScope && isRoot {
		return appendInScopeNamespaceAttrs(attrs, b.reader, scopeDepth)
	}
	return appendScopeNamespaceAttrs(attrs, b.reader, scopeDepth)
}

func parseDOM(reader *xmlstream.Reader, doc *Document, mode parseMode, namespaceMode namespaceAttrsMode, start xmlstream.Event) error {
	builder := newDOMBuilder(doc, reader, namespaceMode)
	docState := value.NewDocumentState()
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
			namespace: value.XMLNSNamespace,
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
			namespace: value.XMLNSNamespace,
			local:     local,
			value:     decl.URI,
		})
	}
	return attrs
}
