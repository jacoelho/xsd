package schemaast

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/value"
)

func (p *documentParser) xsdChildren(elem NodeID) []NodeID {
	children := p.doc.Children(elem)
	if len(children) == 0 {
		return nil
	}
	out := make([]NodeID, 0, len(children))
	for _, child := range children {
		if p.doc.NamespaceURI(child) == value.XSDNamespace {
			out = append(out, child)
		}
	}
	return out
}

func (p *documentParser) parseForm(elem NodeID) (FormChoice, bool, error) {
	if !p.hasAttr(elem, "form") {
		return FormDefault, false, nil
	}
	form := p.attr(elem, "form")
	if form == "" {
		return FormDefault, false, fmt.Errorf("invalid form attribute value '': must be 'qualified' or 'unqualified'")
	}
	switch form {
	case "qualified":
		return FormQualified, true, nil
	case "unqualified":
		return FormUnqualified, true, nil
	default:
		return FormDefault, false, fmt.Errorf("invalid form attribute value '%s': must be 'qualified' or 'unqualified'", form)
	}
}

func (p *documentParser) localElementQualified(form FormChoice) bool {
	switch form {
	case FormQualified:
		return true
	case FormUnqualified:
		return false
	default:
		return p.result.Defaults.ElementFormDefault == Qualified
	}
}

func (p *documentParser) localAttributeQualified(form FormChoice) bool {
	switch form {
	case FormQualified:
		return true
	case FormUnqualified:
		return false
	default:
		return p.result.Defaults.AttributeFormDefault == Qualified
	}
}

func (p *documentParser) resolveQName(elem NodeID, raw string, useDefault bool) (QName, error) {
	prefix, local, hasPrefix, err := ParseQName(raw)
	if err != nil {
		return QName{}, err
	}
	if hasPrefix || useDefault {
		ns, ok := p.lookupNamespace(elem, prefix)
		if !ok {
			if !hasPrefix {
				return QName{Local: local}, nil
			}
			return QName{}, fmt.Errorf("namespace prefix %q is not declared", prefix)
		}
		return QName{Namespace: ns, Local: local}, nil
	}
	return QName{Local: local}, nil
}

func (p *documentParser) contextID(elem NodeID) NamespaceContextID {
	bindings := p.namespaceBindings(elem)
	var key strings.Builder
	for _, binding := range bindings {
		key.WriteString(binding.Prefix)
		key.WriteByte('=')
		key.WriteString(binding.URI)
		key.WriteByte(0)
	}
	keyString := key.String()
	if id, ok := p.contextIDs[keyString]; ok {
		return id
	}
	id := NamespaceContextID(len(p.result.NamespaceContexts))
	p.contextIDs[keyString] = id
	p.result.NamespaceContexts = append(p.result.NamespaceContexts, NamespaceContext{Bindings: bindings})
	return id
}

func (p *documentParser) namespaceBindings(elem NodeID) []NamespaceBinding {
	stack := make([]NodeID, 0, 8)
	for n := elem; n != InvalidNode; n = p.doc.Parent(n) {
		stack = append(stack, n)
	}
	bindings := map[string]NamespaceURI{
		"xml": value.XMLNamespace,
	}
	for i := len(stack) - 1; i >= 0; i-- {
		for _, attr := range p.doc.Attributes(stack[i]) {
			prefix, ok := namespaceDeclPrefix(attr)
			if !ok {
				continue
			}
			bindings[prefix] = attr.Value()
		}
	}
	prefixes := make([]string, 0, len(bindings))
	for prefix := range bindings {
		prefixes = append(prefixes, prefix)
	}
	slices.Sort(prefixes)
	out := make([]NamespaceBinding, 0, len(prefixes))
	for _, prefix := range prefixes {
		out = append(out, NamespaceBinding{Prefix: prefix, URI: bindings[prefix]})
	}
	return out
}

func (p *documentParser) lookupNamespace(elem NodeID, prefix string) (NamespaceURI, bool) {
	bindings := p.namespaceBindings(elem)
	for _, binding := range bindings {
		if binding.Prefix == prefix {
			return binding.URI, true
		}
	}
	return NamespaceEmpty, false
}

func (p *documentParser) attr(elem NodeID, name string) string {
	return ApplyWhiteSpace(p.doc.GetAttribute(elem, name), WhiteSpaceCollapse)
}

func (p *documentParser) rawAttr(elem NodeID, name string) string {
	return p.doc.GetAttribute(elem, name)
}

func (p *documentParser) hasAttr(elem NodeID, name string) bool {
	return p.doc.HasAttribute(elem, name)
}

func (p *documentParser) parseOccursAttr(elem NodeID, name string) (Occurs, bool, error) {
	if !p.hasAttr(elem, name) {
		return Occurs{}, false, nil
	}
	if TrimXMLWhitespace(p.rawAttr(elem, name)) == "" {
		return Occurs{}, false, fmt.Errorf("%s attribute cannot be empty", name)
	}
	occ, err := parseOccursValue(name, p.attr(elem, name))
	if err != nil {
		return Occurs{}, false, err
	}
	return occ, true, nil
}

func (p *documentParser) origin(NodeID) string {
	return p.result.Location
}

func namespaceDeclPrefix(attr Attr) (string, bool) {
	if !isXMLNSDeclaration(attr) {
		return "", false
	}
	if attr.LocalName() == "xmlns" {
		return "", true
	}
	return attr.LocalName(), true
}

func parseSchemaForm(value string) (Form, error) {
	switch ApplyWhiteSpace(value, WhiteSpaceCollapse) {
	case "qualified":
		return Qualified, nil
	case "unqualified":
		return Unqualified, nil
	default:
		return Unqualified, fmt.Errorf("must be 'qualified' or 'unqualified'")
	}
}

func (p *documentParser) parseBoolAttrDefault(elem NodeID, name string, def bool) (bool, error) {
	if !p.hasAttr(elem, name) {
		return def, nil
	}
	return parseBoolValue(name, p.attr(elem, name))
}
