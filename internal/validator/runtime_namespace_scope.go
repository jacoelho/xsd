package validator

import "github.com/jacoelho/xsd/internal/xmlstream"

func (s *Session) pushNamespaceScope(decls []xmlstream.NamespaceDecl) {
	if s == nil {
		return
	}
	s.Names.PushNamespaceScope(decls)
}

func (s *Session) popNamespaceScope() {
	if s == nil {
		return
	}
	s.Names.PopNamespaceScope()
}
