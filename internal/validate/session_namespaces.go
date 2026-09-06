package validate

import (
	"encoding/xml"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *session) runtimeName(n xml.Name) xsdSchema.RuntimeName {
	return ResolveRuntimeName(s.rt, n)
}

func (s *session) simpleValueQNameResolver(id xsdSchema.SimpleTypeID) value.Resolver {
	program := s.rt.ValueProgram()
	needs, ok := program.NeedsQNameResolver(id)
	if !ok || !needs {
		return value.Resolver{}
	}
	return value.Resolver{
		QName:    s.qnameResolver(),
		Notation: s.rt.NotationDeclared,
	}
}

func (s *session) qnameResolver() xsdSchema.ResolveQNameParts {
	if s.resolveLexicalQNamePartsFunc == nil {
		s.resolveLexicalQNamePartsFunc = s.resolveLexicalQNameParts
	}
	return s.resolveLexicalQNamePartsFunc
}

func (s *session) resolveLexicalQNameParts(v string) (namespace, local string, ok bool) {
	return ResolveLexicalQNameParts(v, s.reader.Lookup)
}
