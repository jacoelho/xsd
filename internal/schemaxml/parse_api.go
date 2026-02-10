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

	nodeStack := state.NewStateStack[NodeID](16)
	childCountStack := state.NewStateStack[int](16)
	var attrsScratch []Attr
	rootClosed := false
	allowBOM := true
	for {
		event, err := decoder.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("xml read: %w", err)
		}

		switch event.Kind {
		case xmlstream.EventStartElement:
			if rootClosed {
				return fmt.Errorf("unexpected element %s after document end", event.Name.Local)
			}

			parent := InvalidNode
			if p, ok := nodeStack.Peek(); ok {
				parent = p
			}
			if parent != InvalidNode {
				count, _ := childCountStack.Pop()
				childCountStack.Push(count + 1)
			}
			attrsScratch = attrsScratch[:0]
			for _, attr := range event.Attrs {
				attrsScratch = append(attrsScratch, Attr{
					namespace: attr.Name.Namespace,
					local:     attr.Name.Local,
					value:     string(attr.Value),
				})
			}
			for decl := range decoder.NamespaceDeclsSeq(event.ScopeDepth) {
				local := decl.Prefix
				if local == "" {
					local = "xmlns"
				}
				attrsScratch = append(attrsScratch, Attr{
					namespace: XMLNSNamespace,
					local:     local,
					value:     decl.URI,
				})
			}
			id := doc.addNode(event.Name.Namespace, event.Name.Local, attrsScratch, parent)
			if parent == InvalidNode {
				doc.root = id
				allowBOM = false
			}
			nodeStack.Push(id)
			childCountStack.Push(0)

		case xmlstream.EventEndElement:
			if _, ok := nodeStack.Pop(); ok {
				_, _ = childCountStack.Pop()
				if nodeStack.Len() == 0 && doc.root != InvalidNode {
					rootClosed = true
				}
			}

		case xmlstream.EventCharData:
			if nodeStack.Len() == 0 {
				if !xmllex.IsIgnorableOutsideRoot(event.Text, allowBOM) {
					return fmt.Errorf("unexpected character data outside root element")
				}
				allowBOM = false
				continue
			}
			nodeID, _ := nodeStack.Peek()
			node := &doc.nodes[nodeID]
			textOff := len(node.text)
			node.text = append(node.text, event.Text...)
			count, _ := childCountStack.Peek()
			doc.addTextSegment(nodeID, count, textOff, len(event.Text))
		case xmlstream.EventComment, xmlstream.EventPI, xmlstream.EventDirective:
			if nodeStack.Len() == 0 {
				allowBOM = false
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

	nodeStack := state.NewStateStack[NodeID](16)
	childCountStack := state.NewStateStack[int](16)
	var attrsScratch []Attr

	addStart := func(event xmlstream.Event) {
		parent := InvalidNode
		if p, ok := nodeStack.Peek(); ok {
			parent = p
		}
		if parent != InvalidNode {
			count, _ := childCountStack.Pop()
			childCountStack.Push(count + 1)
		}
		attrsScratch = attrsScratch[:0]
		for _, attr := range event.Attrs {
			attrsScratch = append(attrsScratch, Attr{
				namespace: attr.Name.Namespace,
				local:     attr.Name.Local,
				value:     string(attr.Value),
			})
		}
		if nodeStack.Len() == 0 {
			attrsScratch = appendInScopeNamespaceAttrs(attrsScratch, reader, event.ScopeDepth)
		} else {
			attrsScratch = appendScopeNamespaceAttrs(attrsScratch, reader, event.ScopeDepth)
		}
		id := doc.addNode(event.Name.Namespace, event.Name.Local, attrsScratch, parent)
		if parent == InvalidNode {
			doc.root = id
		}
		nodeStack.Push(id)
		childCountStack.Push(0)
	}

	addStart(start)
	for depth := 1; depth > 0; {
		event, readErr := reader.Next()
		if readErr != nil {
			return fmt.Errorf("xml read: %w", readErr)
		}

		switch event.Kind {
		case xmlstream.EventStartElement:
			addStart(event)
			depth++

		case xmlstream.EventEndElement:
			if _, ok := nodeStack.Pop(); ok {
				_, _ = childCountStack.Pop()
			}
			depth--

		case xmlstream.EventCharData:
			nodeID, ok := nodeStack.Peek()
			if !ok {
				continue
			}
			node := &doc.nodes[nodeID]
			textOff := len(node.text)
			node.text = append(node.text, event.Text...)
			count, _ := childCountStack.Peek()
			doc.addTextSegment(nodeID, count, textOff, len(event.Text))
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
