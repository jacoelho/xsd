package xsd

import (
	"encoding/xml"
	"errors"
	"slices"
	"strings"
)

func (s *session) rootTypeFromXSIType(attrs []streamAttr, line, col int) (typeID, bool, error) {
	for i := range attrs {
		a := &attrs[i]
		if a.Name.Space != xsiNamespaceURI || a.Name.Local != xsiAttrType {
			continue
		}
		typ, err := s.resolveXSIType(a.stringValue(&s.valueStrings), line, col)
		if err != nil {
			return typeID{}, false, err
		}
		return typ, true, nil
	}
	return typeID{}, false, nil
}

func (s *session) effectiveType(elem elementID, typ typeID, attrs []streamAttr, line, col int) (typeID, bool, error) {
	rt := s.engine.rt
	nilled := false
	nilSpecified := false
	for i := range attrs {
		a := &attrs[i]
		if a.Name.Space != xsiNamespaceURI {
			continue
		}
		value := a.stringValue(&s.valueStrings)
		switch a.Name.Local {
		case xsiAttrNil:
			nilSpecified = true
			value, ok := parseBooleanLexical(normalizeWhitespace(value, whitespaceCollapse))
			if !ok {
				return typ, false, validation(ErrValidationNil, line, col, s.pathString(), "invalid xsi:nil value")
			}
			nilled = value
		case xsiAttrType:
			override, err := s.resolveXSIType(value, line, col)
			if err != nil {
				return typ, nilled, err
			}
			if err := s.validateXSITypeOverride(elem, typ, override, line, col); err != nil {
				return typ, nilled, err
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

func (s *session) validateXSITypeOverride(elem elementID, declared, override typeID, line, col int) error {
	rt := s.engine.rt
	mask, ok := rt.typeDerivationMask(override, declared)
	if !ok {
		return validation(ErrValidationType, line, col, s.pathString(), "xsi:type is not derived from declared type")
	}
	if elem == noElement || override == declared {
		return nil
	}
	block := rt.Elements[elem].Block
	if declared.Kind == typeComplex {
		block |= rt.ComplexTypes[declared.ID].Block
	}
	if block&blockExtension != 0 && mask&blockExtension != 0 {
		return validation(ErrValidationType, line, col, s.pathString(), "xsi:type extension is blocked")
	}
	if block&blockRestriction != 0 && mask&blockRestriction != 0 {
		return validation(ErrValidationType, line, col, s.pathString(), "xsi:type restriction is blocked")
	}
	return nil
}

func (s *session) resolveXSIType(value string, line, col int) (typeID, error) {
	q, ok := s.resolveLexicalQName(value)
	if !ok {
		if ns, _, nsOK := s.resolveLexicalQNameParts(value); nsOK && s.hasSchemaLocationHint(ns) {
			return typeID{}, s.unsupportedSchemaLocation(line, col, xsiAttrType, runtimeName{NS: ns, Local: value})
		}
		return typeID{}, validation(ErrValidationType, line, col, s.pathString(), "unknown xsi:type "+value)
	}
	if typ, ok := s.engine.rt.GlobalTypes[q]; ok {
		return typ, nil
	}
	ns := s.engine.rt.Names.Namespace(q.Namespace)
	if s.hasSchemaLocationHint(ns) {
		return typeID{}, s.unsupportedSchemaLocation(line, col, xsiAttrType, runtimeName{Name: q, Known: true, NS: ns, Local: value})
	}
	return typeID{}, validation(ErrValidationType, line, col, s.pathString(), "unknown xsi:type "+value)
}

func (s *session) translateStartElement(se streamStartElement, line, col int) (streamStartElement, error) {
	name, err := s.translateName(se.Name, xmlElementName, line, col)
	if err != nil {
		return streamStartElement{}, err
	}
	se.Name = name
	for i, attr := range se.Attr {
		if isNamespaceName(attr.Name) {
			continue
		}
		name, err := s.translateName(attr.Name, xmlAttributeName, line, col)
		if err != nil {
			return streamStartElement{}, err
		}
		se.Attr[i].Name = name
	}
	if err := validateUniqueStreamAttributeNames(se.Attr); err != nil {
		return streamStartElement{}, validation(ErrValidationXML, line, col, s.pathString(), err.Error())
	}
	return se, nil
}

func validateUniqueStreamAttributeNames(attrs []streamAttr) error {
	if len(attrs) < 2 {
		return nil
	}
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

func (s *session) translateName(name xml.Name, kind xmlNameKind, line, col int) (xml.Name, error) {
	resolved, ok := s.ns.resolveName(name, kind)
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
	return runtimeName{Known: false, NS: n.Space, Local: n.Local}
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
	mark := len(ns.bindings)
	for _, a := range attrs {
		if a.Name.Space == xmlnsPrefix {
			if err := validateNamespaceBinding(a.Name.Local, a.Value); err != nil {
				clear(ns.bindings[mark:])
				ns.bindings = ns.bindings[:mark]
				return err
			}
			ns.bindings = append(ns.bindings, namespaceBinding{Prefix: a.Name.Local, URI: a.Value})
		} else if a.Name.Space == "" && a.Name.Local == xmlnsPrefix {
			if err := validateDefaultNamespaceBinding(a.Value); err != nil {
				clear(ns.bindings[mark:])
				ns.bindings = ns.bindings[:mark]
				return err
			}
			ns.bindings = append(ns.bindings, namespaceBinding{Prefix: "", URI: a.Value})
		}
	}
	ns.frames = append(ns.frames, mark)
	return nil
}

func (s *session) pushNamespaces(attrs []streamAttr) error {
	mark := len(s.ns.bindings)
	for i := range attrs {
		a := &attrs[i]
		if a.Name.Space == xmlnsPrefix {
			value := a.stringValue(&s.valueStrings)
			if err := validateNamespaceBinding(a.Name.Local, value); err != nil {
				clear(s.ns.bindings[mark:])
				s.ns.bindings = s.ns.bindings[:mark]
				return err
			}
			s.ns.bindings = append(s.ns.bindings, namespaceBinding{Prefix: a.Name.Local, URI: value})
		} else if a.Name.Space == "" && a.Name.Local == xmlnsPrefix {
			value := a.stringValue(&s.valueStrings)
			if err := validateDefaultNamespaceBinding(value); err != nil {
				clear(s.ns.bindings[mark:])
				s.ns.bindings = s.ns.bindings[:mark]
				return err
			}
			s.ns.bindings = append(s.ns.bindings, namespaceBinding{Prefix: "", URI: value})
		}
	}
	s.ns.frames = append(s.ns.frames, mark)
	return nil
}

type xmlNameKind uint8

const (
	xmlElementName xmlNameKind = iota
	xmlAttributeName
)

func (ns *namespaceStack) resolveName(name xml.Name, kind xmlNameKind) (xml.Name, bool) {
	if name.Space != "" {
		uri, ok := ns.lookup(name.Space)
		if !ok {
			return xml.Name{}, false
		}
		return xml.Name{Space: uri, Local: name.Local}, true
	}
	if kind == xmlElementName {
		if len(ns.bindings) == 0 {
			return name, true
		}
		uri, _ := ns.lookup("")
		return xml.Name{Space: uri, Local: name.Local}, true
	}
	return name, true
}

func validateNamespaceBinding(prefix, uri string) error {
	if prefix == xmlnsPrefix {
		return errors.New("xmlns prefix cannot be declared")
	}
	if prefix == xmlPrefix {
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
	if prefix == xmlPrefix {
		return xmlNamespaceURI, true
	}
	for _, binding := range slices.Backward(ns.bindings) {
		if binding.Prefix == prefix {
			return binding.URI, true
		}
	}
	if prefix == "" {
		return "", true
	}
	return "", false
}

func isNamespaceAttr(a xml.Attr) bool {
	return isNamespaceName(a.Name)
}

func isNamespaceName(name xml.Name) bool {
	return name.Space == xmlnsPrefix || (name.Space == "" && name.Local == xmlnsPrefix)
}

func isXSIName(name xml.Name) bool {
	return name.Space == xsiNamespaceURI &&
		(name.Local == xsiAttrType || name.Local == xsiAttrNil || name.Local == xsiAttrSchemaLocation || name.Local == xsiAttrNoNamespaceSchemaLocation)
}

func (s *session) pushPath(local string) {
	s.path = append(s.path, local)
}

func (s *session) cachedChildPath(parent, local string) string {
	key := pathCacheKey{Parent: parent, Local: local}
	if path, ok := s.pathCache[key]; ok {
		return path
	}
	path := "/" + local
	if parent != "" {
		path = parent + path
	}
	if len(s.pathCache) < maxRetainedMapLen {
		if s.pathCache == nil {
			s.pathCache = make(map[pathCacheKey]string)
		}
		s.pathCache[key] = path
	}
	return path
}

func (s *session) popPath() {
	if len(s.path) > 0 {
		s.path = s.path[:len(s.path)-1]
	}
	if s.pathTextDepth > len(s.path) {
		s.pathText = ""
		s.pathTextDepth = 0
	}
}

func (s *session) pathString() string {
	if len(s.path) == 0 {
		return "/"
	}
	if s.pathText != "" && s.pathTextDepth == len(s.path) {
		return s.pathText
	}
	parent := ""
	start := 0
	if s.pathText != "" && s.pathTextDepth > 0 && s.pathTextDepth < len(s.path) {
		parent = s.pathText
		start = s.pathTextDepth
	}
	for i := start; i < len(s.path); i++ {
		parent = s.cachedChildPath(parent, s.path[i])
	}
	s.pathText = parent
	s.pathTextDepth = len(s.path)
	return parent
}
