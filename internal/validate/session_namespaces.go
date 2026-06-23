package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

func (s *session) translateStartElement(se stream.StartElement, line, col int) (stream.StartElement, error) {
	name, err := s.translateName(se.Name, xmlElementName, line, col)
	if err != nil {
		return stream.StartElement{}, err
	}
	se.Name = name
	for i, attr := range se.Attr {
		if xmlns.IsNamespaceName(attr.Name) {
			continue
		}
		if attr.Name.Space == "" {
			continue
		}
		name, err := s.translateName(attr.Name, xmlAttributeName, line, col)
		if err != nil {
			return stream.StartElement{}, err
		}
		se.Attr[i].Name = name
	}
	if err := xmlns.ValidateUniqueAttributes(se.Attr); err != nil {
		return stream.StartElement{}, xsderrors.Validation(xsderrors.CodeValidationXML, line, col, s.pathString(), err.Error())
	}
	return se, nil
}

func (s *session) translateName(name xml.Name, kind xmlNameKind, line, col int) (xml.Name, error) {
	resolved, ok := s.doc.ns.ResolveName(name, xmlns.NameKind(kind))
	if !ok {
		return xml.Name{}, validation(s.startContext(line, col), xsderrors.CodeValidationXML, "unbound namespace prefix "+name.Space)
	}
	return resolved, nil
}

func (s *session) runtimeName(n xml.Name) runtime.RuntimeName {
	if s.schema != nil {
		q, ok := s.schema.NameReads.LookupQName(n.Space, n.Local)
		if ok {
			return runtime.RuntimeName{Name: q, Known: true, NS: n.Space, Local: n.Local}
		}
		return runtime.RuntimeName{Known: false, NS: n.Space, Local: n.Local}
	}
	return ResolveRuntimeName(s.rt, n)
}

func (s *session) qnameResolverForAttrs(hasXSIType bool) runtime.ResolveQNameParts {
	if !hasXSIType {
		return nil
	}
	return s.resolveLexicalQNamePartsFunc
}

func (s *session) simpleValueQNameResolver(id runtime.SimpleTypeID) runtime.ResolveQNameParts {
	if !s.rt.SimpleValueNeedsQNameResolver(id) {
		return nil
	}
	return s.resolveLexicalQNamePartsFunc
}

func (s *session) resolveLexicalQNameParts(v string) (string, string, bool) {
	return ResolveLexicalQNameParts(v, s.doc.ns.Lookup)
}

func (s *session) pushNamespaces(attrs []stream.Attr) error {
	return s.doc.ns.PushStream(attrs, &s.valueStrings)
}

type xmlNameKind uint8

const (
	xmlElementName xmlNameKind = iota
	xmlAttributeName
)

func (s *session) pushPath(local string) {
	s.doc.path = append(s.doc.path, local)
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
	if len(s.doc.path) > 0 {
		s.doc.path = s.doc.path[:len(s.doc.path)-1]
	}
	if s.doc.pathTextDepth > len(s.doc.path) {
		s.doc.pathText = ""
		s.doc.pathTextDepth = 0
	}
}

func (s *session) pathString() string {
	if len(s.doc.path) == 0 {
		return "/"
	}
	if s.doc.pathText != "" && s.doc.pathTextDepth == len(s.doc.path) {
		return s.doc.pathText
	}
	parent := ""
	start := 0
	if s.doc.pathText != "" && s.doc.pathTextDepth > 0 && s.doc.pathTextDepth < len(s.doc.path) {
		parent = s.doc.pathText
		start = s.doc.pathTextDepth
	}
	for i := start; i < len(s.doc.path); i++ {
		parent = s.cachedChildPath(parent, s.doc.path[i])
	}
	s.doc.pathText = parent
	s.doc.pathTextDepth = len(s.doc.path)
	return parent
}
