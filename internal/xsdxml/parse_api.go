package xsdxml

import (
	"errors"
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/state"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Parse builds the minimal DOM used by the validator from XML input.
func Parse(r io.Reader) (*Document, error) {
	doc := &Document{root: InvalidNode}
	if err := ParseInto(r, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// ParseInto builds the minimal DOM into an existing document.
func ParseInto(r io.Reader, doc *Document) error {
	return ParseIntoWithOptions(r, doc)
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
			for _, decl := range decoder.NamespaceDeclsAt(event.ScopeDepth) {
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
				if !IsIgnorableOutsideRoot(event.Text, allowBOM) {
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
