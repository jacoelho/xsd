package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) pushNamespaceScope(decls []xmlstream.NamespaceDecl) {
	off := len(s.nsDecls)
	cacheOff := len(s.prefixCache)
	declLen := 0
	for _, decl := range decls {
		declLen++
		prefixOff := len(s.nameLocal)
		s.nameLocal = append(s.nameLocal, decl.Prefix...)
		prefixLen := len(decl.Prefix)
		nsOff := len(s.nameNS)
		s.nameNS = append(s.nameNS, decl.URI...)
		nsLen := len(decl.URI)
		prefixBytes := s.nameLocal[prefixOff : prefixOff+prefixLen]
		s.nsDecls = append(s.nsDecls, nsDecl{
			prefixOff:  uint32(prefixOff),
			prefixLen:  uint32(prefixLen),
			nsOff:      uint32(nsOff),
			nsLen:      uint32(nsLen),
			prefixHash: runtime.HashBytes(prefixBytes),
		})
	}
	s.nsStack.Push(nsFrame{off: uint32(off), len: uint32(declLen), cacheOff: uint32(cacheOff)})
}

func (s *Session) popNamespaceScope() {
	frame, ok := s.nsStack.Pop()
	if !ok {
		return
	}
	if int(frame.off) <= len(s.nsDecls) {
		s.nsDecls = s.nsDecls[:frame.off]
	}
	if int(frame.cacheOff) <= len(s.prefixCache) {
		s.prefixCache = s.prefixCache[:frame.cacheOff]
	}
}
