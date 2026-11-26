package validate

import (
	"encoding/xml"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *session) runtimeName(n xml.Name) xsdSchema.RuntimeName {
	return s.rt.ResolveName(n.Space, n.Local)
}

func (s *session) simpleValueQNameResolver(input value.InputRequirements) value.Resolver {
	if !input.NeedsQName {
		return value.Resolver{}
	}
	if s.valueResolver.Notation == nil {
		s.valueResolver.QName = s.qnameResolver()
		s.valueResolver.Notation = s.rt.NotationDeclared
	}
	return s.valueResolver
}

func (s *session) qnameResolver() value.QNameResolver {
	if s.valueResolver.QName == nil {
		s.valueResolver.QName = s.resolveLexicalQName
	}
	return s.valueResolver.QName
}

func (s *session) resolveLexicalQName(v string) (value.ExpandedName, bool) {
	return resolveLexicalQName(v, s.reader.Lookup)
}
