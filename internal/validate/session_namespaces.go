package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *session) runtimeName(n xml.Name) runtime.RuntimeName {
	return ResolveRuntimeName(s.rt, n)
}

func (s *session) qnameResolverForAttrs(hasXSIType bool) runtime.ResolveQNameParts {
	if !hasXSIType {
		return nil
	}
	return s.qnameResolver()
}

func (s *session) simpleValueQNameResolver(id runtime.SimpleTypeID) runtime.ResolveQNameParts {
	if !s.rt.SimpleValueNeedsQNameResolver(id) {
		return nil
	}
	return s.qnameResolver()
}

func (s *session) qnameResolver() runtime.ResolveQNameParts {
	if s.resolveLexicalQNamePartsFunc == nil {
		s.resolveLexicalQNamePartsFunc = s.resolveLexicalQNameParts
	}
	return s.resolveLexicalQNamePartsFunc
}

func (s *session) resolveLexicalQNameParts(v string) (string, string, bool) {
	return ResolveLexicalQNameParts(v, s.doc.LookupNamespace)
}
