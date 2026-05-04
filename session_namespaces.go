package xsd

import (
	"encoding/xml"
	"errors"
	"strings"
)

func (s *session) rootTypeFromXSIType(attrs []xml.Attr, line, col int) (typeID, bool, error) {
	for _, a := range attrs {
		if a.Name.Space != xsiNamespaceURI || a.Name.Local != "type" {
			continue
		}
		q, ok := s.resolveLexicalQName(a.Value)
		if !ok {
			if ns, _, nsOK := s.resolveLexicalQNameParts(a.Value); nsOK && s.hasSchemaLocationHint(ns) {
				return typeID{}, false, s.unsupportedSchemaLocation(line, col, "type", runtimeName{NS: ns, Local: a.Value})
			}
			return typeID{}, false, validation(ErrValidationType, line, col, s.pathString(), "unknown xsi:type "+a.Value)
		}
		typ, ok := s.engine.rt.GlobalTypes[q]
		if !ok {
			if s.hasSchemaLocationHint(s.engine.rt.Names.Namespace(q.Namespace)) {
				return typeID{}, false, s.unsupportedSchemaLocation(line, col, "type", runtimeName{Name: q, Known: true, NS: s.engine.rt.Names.Namespace(q.Namespace), Local: a.Value})
			}
			return typeID{}, false, validation(ErrValidationType, line, col, s.pathString(), "unknown xsi:type "+a.Value)
		}
		return typ, true, nil
	}
	return typeID{}, false, nil
}

func (s *session) effectiveType(elem elementID, typ typeID, attrs []xml.Attr, line, col int) (typeID, bool, error) {
	rt := s.engine.rt
	nilled := false
	nilSpecified := false
	for _, a := range attrs {
		if a.Name.Space != xsiNamespaceURI {
			continue
		}
		switch a.Name.Local {
		case "nil":
			nilSpecified = true
			switch normalizeWhitespace(a.Value, whitespaceCollapse) {
			case "true", "1":
				nilled = true
			case "false", "0":
			default:
				return typ, false, validation(ErrValidationNil, line, col, s.pathString(), "invalid xsi:nil value")
			}
		case "type":
			q, ok := s.resolveLexicalQName(a.Value)
			if !ok {
				if ns, _, nsOK := s.resolveLexicalQNameParts(a.Value); nsOK && s.hasSchemaLocationHint(ns) {
					return typ, nilled, s.unsupportedSchemaLocation(line, col, "type", runtimeName{NS: ns, Local: a.Value})
				}
				return typ, nilled, validation(ErrValidationType, line, col, s.pathString(), "unknown xsi:type "+a.Value)
			}
			override, ok := rt.GlobalTypes[q]
			if !ok {
				return typ, nilled, validation(ErrValidationType, line, col, s.pathString(), "unknown xsi:type "+a.Value)
			}
			if !s.typeDerivesFrom(override, typ) {
				return typ, nilled, validation(ErrValidationType, line, col, s.pathString(), "xsi:type is not derived from declared type")
			}
			if elem != noElement && override != typ {
				mask, ok := s.typeDerivationMask(override, typ)
				if !ok {
					return typ, nilled, validation(ErrValidationType, line, col, s.pathString(), "xsi:type is not derived from declared type")
				}
				block := rt.Elements[elem].Block
				if typ.Kind == typeComplex {
					block |= rt.ComplexTypes[typ.ID].Block
				}
				if block&blockExtension != 0 && mask&blockExtension != 0 {
					return typ, nilled, validation(ErrValidationType, line, col, s.pathString(), "xsi:type extension is blocked")
				}
				if block&blockRestriction != 0 && mask&blockRestriction != 0 {
					return typ, nilled, validation(ErrValidationType, line, col, s.pathString(), "xsi:type restriction is blocked")
				}
			}
			typ = override
		}
	}
	if typ.Kind == typeComplex && rt.ComplexTypes[typ.ID].Abstract {
		return typ, nilled, validation(ErrValidationType, line, col, s.pathString(), "complex type is abstract")
	}
	if nilSpecified && elem != noElement && !rt.Elements[elem].Nillable {
		return typ, nilled, validation(ErrValidationNil, line, col, s.pathString(), "element is not nillable")
	}
	if nilled {
		if elem == noElement {
			return typ, nilled, validation(ErrValidationNil, line, col, s.pathString(), "element is not nillable")
		}
		if rt.Elements[elem].HasFixed {
			return typ, nilled, validation(ErrValidationNil, line, col, s.pathString(), "nilled element cannot have fixed value")
		}
	}
	return typ, nilled, nil
}

func (s *session) typeDerivesFrom(t, base typeID) bool {
	_, ok := s.typeDerivationMask(t, base)
	return ok
}

func (s *session) typeDerivationMask(t, base typeID) (derivationMask, bool) {
	return s.engine.rt.typeDerivationMask(t, base)
}

func (s *session) translateStartElement(se xml.StartElement, line, col int) (xml.StartElement, error) {
	name, err := s.translateName(se.Name, true, line, col)
	if err != nil {
		return xml.StartElement{}, err
	}
	se.Name = name
	for i, attr := range se.Attr {
		if isNamespaceAttr(attr) {
			continue
		}
		name, err := s.translateName(attr.Name, false, line, col)
		if err != nil {
			return xml.StartElement{}, err
		}
		se.Attr[i].Name = name
	}
	if err := validateUniqueAttributeNames(se.Attr); err != nil {
		return xml.StartElement{}, validation(ErrValidationXML, line, col, s.pathString(), err.Error())
	}
	return se, nil
}

func validateUniqueAttributeNames(attrs []xml.Attr) error {
	for i, attr := range attrs {
		for _, other := range attrs[:i] {
			if attr.Name == other.Name {
				return errors.New("duplicate attribute " + formatXMLName(attr.Name))
			}
		}
	}
	return nil
}

func (s *session) translateName(name xml.Name, element bool, line, col int) (xml.Name, error) {
	if name.Space != "" {
		uri, ok := s.ns.lookup(name.Space)
		if !ok {
			return xml.Name{}, validation(ErrValidationXML, line, col, s.pathString(), "unbound namespace prefix "+name.Space)
		}
		return xml.Name{Space: uri, Local: name.Local}, nil
	}
	if element {
		uri, _ := s.ns.lookup("")
		return xml.Name{Space: uri, Local: name.Local}, nil
	}
	return name, nil
}

func (s *session) runtimeName(n xml.Name) runtimeName {
	rt := s.engine.rt
	q, ok := rt.Names.LookupQName(n.Space, n.Local)
	if ok {
		return runtimeName{Name: q, Known: true, NS: n.Space, Local: n.Local}
	}
	return runtimeName{Known: false, NS: n.Space, Local: formatXMLName(n)}
}

func formatXMLName(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	return "{" + n.Space + "}" + n.Local
}

func wildcardMatches(rt *runtimeSchema, w wildcard, rn runtimeName) bool {
	switch w.Mode {
	case wildAny:
		return true
	case wildOther:
		return rn.NS != "" && rn.NS != rt.Names.Namespace(w.OtherThan)
	case wildLocal:
		return rn.NS == ""
	case wildTargetNamespace:
		if len(w.Namespaces) == 0 {
			return false
		}
		return rn.NS == rt.Names.Namespace(w.Namespaces[0])
	case wildList:
		for _, ns := range w.Namespaces {
			if rn.NS == rt.Names.Namespace(ns) {
				return true
			}
		}
	}
	return false
}

func (s *session) resolveLexicalQName(v string) (qName, bool) {
	uri, local, ok := s.resolveLexicalQNameParts(v)
	if !ok {
		return qName{}, false
	}
	return s.engine.rt.Names.LookupQName(uri, local)
}

func (s *session) resolveLexicalQNameParts(v string) (string, string, bool) {
	v = normalizeWhitespace(v, whitespaceCollapse)
	prefix, local, ok := strings.Cut(v, ":")
	if !ok {
		local = v
		prefix = ""
	}
	if ok && prefix == "" {
		return "", "", false
	}
	if local == "" || strings.Contains(local, ":") || !isNCName(local) {
		return "", "", false
	}
	if prefix != "" && !isNCName(prefix) {
		return "", "", false
	}
	uri, ok := s.ns.lookup(prefix)
	if !ok {
		return "", "", false
	}
	return uri, local, true
}

func (s *session) resolveLexicalQNameValue(v string) (string, bool) {
	v = normalizeWhitespace(v, whitespaceCollapse)
	prefix, local, ok := strings.Cut(v, ":")
	if !ok {
		local = v
		prefix = ""
	}
	if ok && prefix == "" {
		return "", false
	}
	if !isNCName(local) || strings.Contains(local, ":") {
		return "", false
	}
	if prefix != "" && !isNCName(prefix) {
		return "", false
	}
	uri, ok := s.ns.lookup(prefix)
	if !ok {
		return "", false
	}
	if uri == "" {
		return local, true
	}
	return "{" + uri + "}" + local, true
}

func (ns *namespaceStack) push(attrs []xml.Attr) error {
	pending := make([]namespaceBinding, 0, len(attrs))
	for _, a := range attrs {
		if a.Name.Space == "xmlns" {
			if err := validateNamespaceBinding(a.Name.Local, a.Value); err != nil {
				return err
			}
			pending = append(pending, namespaceBinding{Prefix: a.Name.Local, URI: a.Value})
		} else if a.Name.Space == "" && a.Name.Local == "xmlns" {
			if err := validateDefaultNamespaceBinding(a.Value); err != nil {
				return err
			}
			pending = append(pending, namespaceBinding{Prefix: "", URI: a.Value})
		}
	}
	ns.frames = append(ns.frames, len(ns.bindings))
	ns.bindings = append(ns.bindings, pending...)
	return nil
}

func validateNamespaceBinding(prefix, uri string) error {
	if prefix == "xmlns" {
		return errors.New("xmlns prefix cannot be declared")
	}
	if prefix == "xml" {
		if uri != xmlNamespaceURI {
			return errors.New("xml prefix must be bound to " + xmlNamespaceURI)
		}
		return nil
	}
	if uri == "" {
		return errors.New("prefixed namespace binding cannot be empty")
	}
	if uri == xmlNamespaceURI {
		return errors.New("xml namespace URI can only be bound to xml prefix")
	}
	if uri == xmlnsNamespaceURI {
		return errors.New("xmlns namespace URI cannot be declared")
	}
	return nil
}

func validateDefaultNamespaceBinding(uri string) error {
	if uri == xmlNamespaceURI {
		return errors.New("xml namespace URI cannot be the default namespace")
	}
	if uri == xmlnsNamespaceURI {
		return errors.New("xmlns namespace URI cannot be declared")
	}
	return nil
}

func (ns *namespaceStack) pop() {
	if len(ns.frames) == 0 {
		return
	}
	mark := ns.frames[len(ns.frames)-1]
	ns.frames = ns.frames[:len(ns.frames)-1]
	ns.bindings = ns.bindings[:mark]
}

func (ns *namespaceStack) lookup(prefix string) (string, bool) {
	if prefix == "xml" {
		return xmlNamespaceURI, true
	}
	for i := len(ns.bindings) - 1; i >= 0; i-- {
		if ns.bindings[i].Prefix == prefix {
			return ns.bindings[i].URI, true
		}
	}
	if prefix == "" {
		return "", true
	}
	return "", false
}

func isNamespaceAttr(a xml.Attr) bool {
	return a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns")
}

func isXSIAttr(a xml.Attr) bool {
	return a.Name.Space == xsiNamespaceURI &&
		(a.Name.Local == "type" || a.Name.Local == "nil" || a.Name.Local == "schemaLocation" || a.Name.Local == "noNamespaceSchemaLocation")
}

func (s *session) pathString() string {
	if len(s.path) == 0 {
		return "/"
	}
	return "/" + strings.Join(s.path, "/")
}
