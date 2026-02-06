package xsdxml

import "strings"

// NodeID identifies a node in the document arena.
type NodeID int

// InvalidNode represents an invalid node reference.
const InvalidNode NodeID = -1

// Document is a compact arena for parsed XML.
type Document struct {
	nodes         []node
	attrs         []Attr
	children      []NodeID
	textSegments  []textSegment
	textScratch   []textScratchEntry
	countsScratch []int
	root          NodeID
}

type node struct {
	namespace   string
	local       string
	text        []byte
	attrsOff    int
	attrsLen    int
	childrenOff int
	childrenLen int
	textSegOff  int
	textSegLen  int
	parent      NodeID
}

type textSegment struct {
	childIndex int
	textOff    int
	textLen    int
}

type textScratchEntry struct {
	parent     NodeID
	childIndex int
	textOff    int
	textLen    int
}

// Attr exposes attribute name, namespace, and value.
type Attr struct {
	namespace string
	local     string
	value     string
}

func (a Attr) Name() string {
	return a.local
}

func (a Attr) NamespaceURI() string {
	return a.namespace
}

func (a Attr) LocalName() string {
	return a.local
}

func (a Attr) Value() string {
	return a.value
}

func (d *Document) reset() {
	if d == nil {
		return
	}
	d.nodes = d.nodes[:0]
	d.attrs = d.attrs[:0]
	d.children = d.children[:0]
	d.textSegments = d.textSegments[:0]
	d.textScratch = d.textScratch[:0]
	d.countsScratch = d.countsScratch[:0]
	d.root = InvalidNode
}

// DocumentElement returns the document root node.
func (d *Document) DocumentElement() NodeID {
	if d == nil {
		return InvalidNode
	}
	return d.root
}

// Root returns the document root node.
func (d *Document) Root() NodeID {
	return d.DocumentElement()
}

func (d *Document) validNode(id NodeID) bool {
	return d != nil && id >= 0 && int(id) < len(d.nodes)
}

// Parent returns the parent node of id, or InvalidNode for the root.
func (d *Document) Parent(id NodeID) NodeID {
	if !d.validNode(id) {
		return InvalidNode
	}
	return d.nodes[id].parent
}

// NamespaceURI returns the namespace URI for the given node.
func (d *Document) NamespaceURI(id NodeID) string {
	if !d.validNode(id) {
		return ""
	}
	return d.nodes[id].namespace
}

// LocalName returns the local name for the given node.
func (d *Document) LocalName(id NodeID) string {
	if !d.validNode(id) {
		return ""
	}
	return d.nodes[id].local
}

// Attributes returns a read-only view of the element attributes.
// The returned slice aliases the document arena; do not modify or retain it.
func (d *Document) Attributes(id NodeID) []Attr {
	if !d.validNode(id) {
		return nil
	}
	n := d.nodes[id]
	if n.attrsLen == 0 {
		return nil
	}
	return d.attrs[n.attrsOff : n.attrsOff+n.attrsLen]
}

// Children returns a read-only view of the element children.
// The returned slice aliases the document arena; do not modify or retain it.
func (d *Document) Children(id NodeID) []NodeID {
	if !d.validNode(id) {
		return nil
	}
	n := d.nodes[id]
	if n.childrenLen == 0 {
		return nil
	}
	return d.children[n.childrenOff : n.childrenOff+n.childrenLen]
}

// DirectTextContent returns only the text directly under the element.
func (d *Document) DirectTextContent(id NodeID) string {
	if !d.validNode(id) {
		return ""
	}
	return string(d.nodes[id].text)
}

// DirectTextContentBytes returns only the text directly under the element as bytes.
// The returned slice aliases the document arena; do not modify or retain it.
func (d *Document) DirectTextContentBytes(id NodeID) []byte {
	if !d.validNode(id) {
		return nil
	}
	return d.nodes[id].text
}

// TextContent returns the concatenated text content of the element subtree.
func (d *Document) TextContent(id NodeID) string {
	if !d.validNode(id) {
		return ""
	}
	n := d.nodes[id]
	if n.childrenLen == 0 {
		return string(n.text)
	}
	var sb strings.Builder
	d.collectText(id, &sb)
	return sb.String()
}

func (d *Document) collectText(id NodeID, sb *strings.Builder) {
	n := d.nodes[id]
	if n.childrenLen == 0 {
		_, _ = sb.Write(n.text)
		return
	}
	children := d.Children(id)
	if n.textSegLen == 0 {
		for _, child := range children {
			d.collectText(child, sb)
		}
		return
	}
	segments := d.textSegments[n.textSegOff : n.textSegOff+n.textSegLen]
	childIdx := 0
	for _, segment := range segments {
		for childIdx < segment.childIndex && childIdx < len(children) {
			d.collectText(children[childIdx], sb)
			childIdx++
		}
		_, _ = sb.Write(n.text[segment.textOff : segment.textOff+segment.textLen])
	}
	for childIdx < len(children) {
		d.collectText(children[childIdx], sb)
		childIdx++
	}
}

func (d *Document) findAttribute(id NodeID, match func(Attr) bool) (Attr, bool) {
	if !d.validNode(id) {
		return Attr{}, false
	}
	for _, attr := range d.Attributes(id) {
		if match(attr) {
			return attr, true
		}
	}
	return Attr{}, false
}

// GetAttribute returns the value of an unqualified attribute name.
func (d *Document) GetAttribute(id NodeID, name string) string {
	if attr, ok := d.findAttribute(id, func(a Attr) bool { return a.namespace == "" && a.local == name }); ok {
		return attr.value
	}
	return ""
}

// GetAttributeNS returns the value of a namespaced attribute.
func (d *Document) GetAttributeNS(id NodeID, ns, local string) string {
	if attr, ok := d.findAttribute(id, func(a Attr) bool { return a.namespace == ns && a.local == local }); ok {
		return attr.value
	}
	return ""
}

// HasAttribute reports whether the element has an unqualified attribute name.
func (d *Document) HasAttribute(id NodeID, name string) bool {
	_, ok := d.findAttribute(id, func(a Attr) bool { return a.namespace == "" && a.local == name })
	return ok
}

// HasAttributeNS reports whether the element has a namespaced attribute.
func (d *Document) HasAttributeNS(id NodeID, ns, local string) bool {
	_, ok := d.findAttribute(id, func(a Attr) bool { return a.namespace == ns && a.local == local })
	return ok
}
