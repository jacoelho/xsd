package xsdxml

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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
	if doc == nil {
		return fmt.Errorf("nil XML document")
	}

	decoder, err := xmlstream.NewReader(r)
	if err != nil {
		return err
	}

	doc.reset()
	var stack []NodeID
	var attrsScratch []Attr
	rootClosed := false
	for {
		event, err := decoder.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		switch event.Kind {
		case xmlstream.EventStartElement:
			if rootClosed {
				return fmt.Errorf("unexpected element %s after document end", event.Name.Local)
			}

			parent := InvalidNode
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
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
			}
			stack = append(stack, id)

		case xmlstream.EventEndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
				if len(stack) == 0 && doc.root != InvalidNode {
					rootClosed = true
				}
			}

		case xmlstream.EventCharData:
			if len(stack) == 0 {
				if !isIgnorableOutsideRoot(event.Text) {
					return fmt.Errorf("unexpected character data outside root element")
				}
				continue
			}
			nodeID := stack[len(stack)-1]
			doc.nodes[nodeID].text = append(doc.nodes[nodeID].text, event.Text...)
		}
	}

	if doc.root == InvalidNode {
		return io.ErrUnexpectedEOF
	}

	doc.buildChildren()
	return nil
}

func (d *Document) addNode(namespace, local string, attrs []Attr, parent NodeID) NodeID {
	id := NodeID(len(d.nodes))

	attrsOff := len(d.attrs)
	if len(attrs) > 0 {
		d.attrs = slices.Grow(d.attrs, len(attrs))
		d.attrs = d.attrs[:attrsOff+len(attrs)]
		for i, attr := range attrs {
			copied := attr
			copied.value = strings.Clone(attr.value)
			d.attrs[attrsOff+i] = copied
		}
	}

	d.nodes = append(d.nodes, node{
		namespace: namespace,
		local:     local,
		attrsOff:  attrsOff,
		attrsLen:  len(attrs),
		parent:    parent,
	})

	return id
}

func (d *Document) buildChildren() {
	if len(d.nodes) == 0 {
		return
	}

	counts := d.countsScratch
	if cap(counts) < len(d.nodes) {
		counts = make([]int, len(d.nodes))
	} else {
		counts = counts[:len(d.nodes)]
		clear(counts)
	}
	d.countsScratch = counts
	for i := range d.nodes {
		parent := d.nodes[i].parent
		if parent != InvalidNode {
			counts[parent]++
		}
	}

	total := 0
	for i := range counts {
		count := counts[i]
		d.nodes[i].childrenOff = total
		d.nodes[i].childrenLen = count
		counts[i] = total
		total += count
	}

	if total == 0 {
		d.children = d.children[:0]
		return
	}

	if cap(d.children) < total {
		d.children = make([]NodeID, total)
	} else {
		d.children = d.children[:total]
	}

	for i := range d.nodes {
		parent := d.nodes[i].parent
		if parent == InvalidNode {
			continue
		}
		idx := counts[parent]
		d.children[idx] = NodeID(i)
		counts[parent]++
	}
}

func isIgnorableOutsideRoot(data []byte) bool {
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if r != '\uFEFF' && !unicode.IsSpace(r) {
			return false
		}
		data = data[size:]
	}
	return true
}
