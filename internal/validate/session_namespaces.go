package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *session) runtimeName(n xml.Name) runtime.RuntimeName {
	return ResolveRuntimeName(s.rt, n)
}

func (s *session) qnameResolverForAttrs(flags xsiStartAttributeFlags) runtime.ResolveQNameParts {
	if !flags.Type {
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

func (s *session) resolveLexicalQNameParts(v string) (namespace, local string, ok bool) {
	return ResolveLexicalQNameParts(v, s.doc.LookupNamespace)
}
