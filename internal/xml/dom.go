package xml

// NodeType classifies nodes in the minimal DOM used by the validator.
type NodeType int

const (
	// ElementNode identifies an element in the parsed document tree.
	ElementNode NodeType = 1
	// TextNode identifies text nodes; text is usually accessed via Element.TextContent.
	TextNode NodeType = 3
	// AttrNode identifies an attribute node when exposed as Attr.
	AttrNode NodeType = 2
)

// Node is the minimal DOM node contract needed by the validator.
type Node interface {
	NodeType() NodeType
	NodeName() string
	NodeValue() string
}

// Document exposes the root element for validation.
type Document interface {
	DocumentElement() Element
}

// Element is the minimal element view used by schema validation.
type Element interface {
	Node
	NamespaceURI() string
	LocalName() string
	Prefix() string
	GetAttribute(name string) string
	GetAttributeNS(ns, local string) string
	HasAttribute(name string) bool
	HasAttributeNS(ns, local string) bool
	Attributes() []Attr
	Children() []Element
	Parent() Element // Parent returns the parent element; nil for the root.
	TextContent() string
	DirectTextContent() string // DirectTextContent returns only text directly under the element.
}

// Attr exposes attribute name, namespace, and value.
type Attr interface {
	Name() string
	NamespaceURI() string
	LocalName() string
	Value() string
}
