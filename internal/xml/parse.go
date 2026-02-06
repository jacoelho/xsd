package xsdxml

import (
	"errors"
	"fmt"
	"io"
	"slices"
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

	var stack []NodeID
	var childCounts []int
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
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			if parent != InvalidNode {
				childCounts[len(childCounts)-1]++
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
			stack = append(stack, id)
			childCounts = append(childCounts, 0)

		case xmlstream.EventEndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
				childCounts = childCounts[:len(childCounts)-1]
				if len(stack) == 0 && doc.root != InvalidNode {
					rootClosed = true
				}
			}

		case xmlstream.EventCharData:
			if len(stack) == 0 {
				if !IsIgnorableOutsideRoot(event.Text, allowBOM) {
					return fmt.Errorf("unexpected character data outside root element")
				}
				allowBOM = false
				continue
			}
			nodeID := stack[len(stack)-1]
			node := &doc.nodes[nodeID]
			textOff := len(node.text)
			node.text = append(node.text, event.Text...)
			doc.addTextSegment(nodeID, childCounts[len(childCounts)-1], textOff, len(event.Text))
		case xmlstream.EventComment, xmlstream.EventPI, xmlstream.EventDirective:
			if len(stack) == 0 {
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

func (d *Document) addNode(namespace, local string, attrs []Attr, parent NodeID) NodeID {
	id := NodeID(len(d.nodes))

	attrsOff := len(d.attrs)
	if len(attrs) > 0 {
		d.attrs = slices.Grow(d.attrs, len(attrs))
		d.attrs = d.attrs[:attrsOff+len(attrs)]
		for i, attr := range attrs {
			d.attrs[attrsOff+i] = attr
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

	counts := d.acquireCountsScratch()
	for i := range d.nodes {
		parent := d.nodes[i].parent
		if parent != InvalidNode {
			counts[parent]++
		}
	}

	total := assignOffsets(counts, func(i, off, count int) {
		d.nodes[i].childrenOff = off
		d.nodes[i].childrenLen = count
	})

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

func (d *Document) addTextSegment(parent NodeID, childIndex, textOff, textLen int) {
	if textLen == 0 {
		return
	}
	if len(d.textScratch) > 0 {
		last := &d.textScratch[len(d.textScratch)-1]
		if last.parent == parent && last.childIndex == childIndex && last.textOff+last.textLen == textOff {
			last.textLen += textLen
			return
		}
	}
	d.textScratch = append(d.textScratch, textScratchEntry{
		parent:     parent,
		childIndex: childIndex,
		textOff:    textOff,
		textLen:    textLen,
	})
}

func (d *Document) buildTextSegments() {
	if len(d.nodes) == 0 || len(d.textScratch) == 0 {
		d.textSegments = d.textSegments[:0]
		return
	}

	counts := d.acquireCountsScratch()

	for _, entry := range d.textScratch {
		if entry.parent == InvalidNode {
			continue
		}
		counts[entry.parent]++
	}

	total := assignOffsets(counts, func(i, off, count int) {
		d.nodes[i].textSegOff = off
		d.nodes[i].textSegLen = count
	})

	if total == 0 {
		d.textSegments = d.textSegments[:0]
		return
	}
	if cap(d.textSegments) < total {
		d.textSegments = make([]textSegment, total)
	} else {
		d.textSegments = d.textSegments[:total]
	}

	for _, entry := range d.textScratch {
		if entry.parent == InvalidNode {
			continue
		}
		idx := counts[entry.parent]
		d.textSegments[idx] = textSegment{
			childIndex: entry.childIndex,
			textOff:    entry.textOff,
			textLen:    entry.textLen,
		}
		counts[entry.parent]++
	}
}

func (d *Document) acquireCountsScratch() []int {
	counts := d.countsScratch
	if cap(counts) < len(d.nodes) {
		counts = make([]int, len(d.nodes))
	} else {
		counts = counts[:len(d.nodes)]
		clear(counts)
	}
	d.countsScratch = counts
	return counts
}

func assignOffsets(counts []int, setOffset func(i, off, count int)) int {
	total := 0
	for i := range counts {
		count := counts[i]
		setOffset(i, total, count)
		counts[i] = total
		total += count
	}
	return total
}

// IsIgnorableOutsideRoot reports whether data contains only XML whitespace.
// If allowBOM is true, a leading BOM is permitted before any other character.
func IsIgnorableOutsideRoot(data []byte, allowBOM bool) bool {
	sawNonBOM := false
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if r == '\uFEFF' {
			if !allowBOM || sawNonBOM {
				return false
			}
			allowBOM = false
			data = data[size:]
			continue
		}
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
		sawNonBOM = true
		allowBOM = false
		data = data[size:]
	}
	return true
}
