package xsdxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"slices"
	"unicode"
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

	decoder := xml.NewDecoder(r)
	if decoder == nil {
		return fmt.Errorf("nil XML decoder")
	}

	doc.reset()
	var stack []NodeID
	rootClosed := false
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if rootClosed {
				return fmt.Errorf("unexpected element %s after document end", t.Name.Local)
			}

			parent := InvalidNode
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			id := doc.addNode(t.Name, t.Attr, parent)
			if parent == InvalidNode {
				doc.root = id
			}
			stack = append(stack, id)

		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
				if len(stack) == 0 && doc.root != InvalidNode {
					rootClosed = true
				}
			}

		case xml.CharData:
			if len(stack) == 0 {
				if !isIgnorableOutsideRoot(string(t)) {
					return fmt.Errorf("unexpected character data outside root element")
				}
				continue
			}
			nodeID := stack[len(stack)-1]
			doc.nodes[nodeID].text = append(doc.nodes[nodeID].text, t...)
		}
	}

	if doc.root == InvalidNode {
		return io.ErrUnexpectedEOF
	}

	doc.buildChildren()
	return nil
}

func (d *Document) addNode(name xml.Name, attrs []xml.Attr, parent NodeID) NodeID {
	id := NodeID(len(d.nodes))

	attrsOff := len(d.attrs)
	if len(attrs) > 0 {
		d.attrs = slices.Grow(d.attrs, len(attrs))
		d.attrs = d.attrs[:attrsOff+len(attrs)]
		for i, a := range attrs {
			namespace := a.Name.Space
			if namespace == "xmlns" || (namespace == "" && a.Name.Local == "xmlns") {
				namespace = XMLNSNamespace
			}
			d.attrs[attrsOff+i] = Attr{
				namespace: namespace,
				local:     a.Name.Local,
				value:     a.Value,
			}
		}
	}

	d.nodes = append(d.nodes, node{
		namespace: name.Space,
		local:     name.Local,
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

func isIgnorableOutsideRoot(data string) bool {
	for _, r := range data {
		if r == '\uFEFF' {
			continue
		}
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
