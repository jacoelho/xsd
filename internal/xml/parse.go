package xml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Parse builds the minimal DOM used by the validator from XML input.
func Parse(r io.Reader) (Document, error) {
	decoder := xml.NewDecoder(r)

	var stack []*element
	var root *element
	rootClosed := false

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if rootClosed {
				return nil, fmt.Errorf("unexpected element %s after document end", t.Name.Local)
			}
			elem := &element{
				namespace: t.Name.Space,
				local:     t.Name.Local,
				attrs:     convertAttrs(t.Attr),
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, elem)
				elem.parent = parent
			} else {
				root = elem
			}
			stack = append(stack, elem)

		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
				if len(stack) == 0 && root != nil {
					rootClosed = true
				}
			}

		case xml.CharData:
			if len(stack) == 0 {
				if !isIgnorableOutsideRoot(string(t)) {
					return nil, fmt.Errorf("unexpected character data outside root element")
				}
				continue
			}
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(t)
			}
		}
	}

	if root == nil {
		return nil, io.ErrUnexpectedEOF
	}

	return &document{root: root}, nil
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

type document struct {
	root *element
}

func (d *document) DocumentElement() Element {
	return d.root
}

type element struct {
	namespace string
	local     string
	attrs     []attr
	children  []*element
	parent    *element
	text      string
}

func (e *element) NodeType() NodeType {
	return ElementNode
}

func (e *element) NodeName() string {
	return e.local
}

func (e *element) NodeValue() string {
	return ""
}

func (e *element) NamespaceURI() string {
	return e.namespace
}

func (e *element) LocalName() string {
	return e.local
}

// Prefix returns an empty string because prefixes are not preserved.
func (e *element) Prefix() string {
	return ""
}

func (e *element) GetAttribute(name string) string {
	for _, attr := range e.attrs {
		if attr.Name() == name {
			return attr.Value()
		}
	}
	return ""
}

func (e *element) GetAttributeNS(ns, local string) string {
	for _, attr := range e.attrs {
		if attr.NamespaceURI() == ns && attr.LocalName() == local {
			return attr.Value()
		}
	}
	return ""
}

func (e *element) HasAttribute(name string) bool {
	for _, attr := range e.attrs {
		if attr.Name() == name {
			return true
		}
	}
	return false
}

func (e *element) HasAttributeNS(ns, local string) bool {
	for _, attr := range e.attrs {
		if attr.NamespaceURI() == ns && attr.LocalName() == local {
			return true
		}
	}
	return false
}

// Attributes returns a copy of the element attributes.
func (e *element) Attributes() []Attr {
	result := make([]Attr, len(e.attrs))
	for i := range e.attrs {
		result[i] = e.attrs[i]
	}
	return result
}

// Children returns a copy of the child element slice.
func (e *element) Children() []Element {
	result := make([]Element, len(e.children))
	for i, child := range e.children {
		result[i] = child
	}
	return result
}

func (e *element) Parent() Element {
	if e.parent == nil {
		return nil
	}
	return e.parent
}

// TextContent returns the concatenated text content of the element subtree.
func (e *element) TextContent() string {
	var sb strings.Builder
	e.collectText(&sb)
	return sb.String()
}

// DirectTextContent returns the direct text content of this element.
func (e *element) DirectTextContent() string {
	return e.text
}

func (e *element) collectText(sb *strings.Builder) {
	sb.WriteString(e.text)
	for _, child := range e.children {
		child.collectText(sb)
	}
}

type attr struct {
	namespace string
	local     string
	value     string
}

func (a attr) Name() string {
	return a.local
}

func (a attr) NamespaceURI() string {
	return a.namespace
}

func (a attr) LocalName() string {
	return a.local
}

func (a attr) Value() string {
	return a.value
}

func convertAttrs(xmlAttrs []xml.Attr) []attr {
	attrs := make([]attr, 0, len(xmlAttrs))
	for _, a := range xmlAttrs {
		namespace := a.Name.Space
		if namespace == "xmlns" || (namespace == "" && a.Name.Local == "xmlns") {
			namespace = XMLNSNamespace
		}
		attrs = append(attrs, attr{
			namespace: namespace,
			local:     a.Name.Local,
			value:     a.Value,
		})
	}
	return attrs
}
