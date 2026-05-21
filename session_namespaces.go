package xsd

import (
	"encoding/xml"
	"errors"
	"slices"
	"strings"
)

func (s *session) rootTypeFromXSIType(attrs []xml.Attr, line, col int) (typeID, bool, error) {
	for _, a := range attrs {
		if a.Name.Space != xsiNamespaceURI || a.Name.Local != "type" {
			continue
		}
		typ, err := s.resolveXSIType(a.Value, line, col)
		if err != nil {
			return typeID{}, false, err
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
			value, ok := parseBooleanLexical(normalizeWhitespace(a.Value, whitespaceCollapse))
			if !ok {
				return typ, false, validation(ErrValidationNil, line, col, s.pathString(), "invalid xsi:nil value")
			}
			nilled = value
		case "type":
			override, err := s.resolveXSIType(a.Value, line, col)
			if err != nil {
				return typ, nilled, err
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

func (s *session) resolveXSIType(value string, line, col int) (typeID, error) {
	q, ok := s.resolveLexicalQName(value)
	if !ok {
		if ns, _, nsOK := s.resolveLexicalQNameParts(value); nsOK && s.hasSchemaLocationHint(ns) {
			return typeID{}, s.unsupportedSchemaLocation(line, col, "type", runtimeName{NS: ns, Local: value})
		}
		return typeID{}, validation(ErrValidationType, line, col, s.pathString(), "unknown xsi:type "+value)
	}
	if typ, ok := s.engine.rt.GlobalTypes[q]; ok {
		return typ, nil
	}
	ns := s.engine.rt.Names.Namespace(q.Namespace)
	if s.hasSchemaLocationHint(ns) {
		return typeID{}, s.unsupportedSchemaLocation(line, col, "type", runtimeName{Name: q, Known: true, NS: ns, Local: value})
	}
	return typeID{}, validation(ErrValidationType, line, col, s.pathString(), "unknown xsi:type "+value)
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
	var seen xmlNameSet
	for _, attr := range attrs {
		if err := addUniqueXMLName(&seen, attr.Name); err != nil {
			return err
		}
	}
	return nil
}

func addUniqueXMLName(seen *xmlNameSet, name xml.Name) error {
	if !seen.add(name) {
		return errors.New("duplicate attribute " + formatXMLName(name))
	}
	return nil
}

const xmlNameSetLinearLimit = 16

type xmlNameSet struct {
	index map[xml.Name]struct{}
	names [xmlNameSetLinearLimit]xml.Name
	n     int
}

func (s *xmlNameSet) add(name xml.Name) bool {
	if s.index != nil {
		if _, ok := s.index[name]; ok {
			return false
		}
		s.index[name] = struct{}{}
		return true
	}
	if slices.Contains(s.names[:s.n], name) {
		return false
	}
	if s.n < len(s.names) {
		s.names[s.n] = name
		s.n++
		return true
	}
	s.index = make(map[xml.Name]struct{}, s.n+1)
	for _, existing := range s.names[:s.n] {
		s.index[existing] = struct{}{}
	}
	s.index[name] = struct{}{}
	return true
}

func (s *session) translateName(name xml.Name, element bool, line, col int) (xml.Name, error) {
	resolved, ok := s.ns.resolveName(name, element)
	if !ok {
		return xml.Name{}, validation(ErrValidationXML, line, col, s.pathString(), "unbound namespace prefix "+name.Space)
	}
	return resolved, nil
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
	return formatExpandedName(n.Space, n.Local)
}

func formatExpandedName(ns, local string) string {
	if ns == "" {
		return local
	}
	return "{" + ns + "}" + local
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
	uri, local, ok := s.resolveLexicalQNameParts(v)
	if !ok {
		return "", false
	}
	return formatExpandedName(uri, local), true
}

func (ns *namespaceStack) push(attrs []xml.Attr) error {
	var pending []namespaceBinding
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

func (ns *namespaceStack) resolveName(name xml.Name, element bool) (xml.Name, bool) {
	if name.Space != "" {
		uri, ok := ns.lookup(name.Space)
		if !ok {
			return xml.Name{}, false
		}
		return xml.Name{Space: uri, Local: name.Local}, true
	}
	if element {
		uri, _ := ns.lookup("")
		return xml.Name{Space: uri, Local: name.Local}, true
	}
	return name, true
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
	i := len(ns.frames) - 1
	mark := ns.frames[i]
	ns.frames[i] = 0
	ns.frames = ns.frames[:i]
	clear(ns.bindings[mark:])
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
	if !s.pathDirty {
		return s.pathText
	}
	s.pathText = "/" + strings.Join(s.path, "/")
	s.pathDirty = false
	return s.pathText
}
