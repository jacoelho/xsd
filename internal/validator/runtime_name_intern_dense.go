package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) internName(id xmlstream.NameID, nsBytes, local []byte) nameEntry {
	if id == 0 {
		return nameEntry{Sym: 0, NS: s.namespaceID(nsBytes)}
	}
	idx := int(id)
	if idx >= maxNameMapSize {
		return s.internSparseName(NameID(id), nsBytes, local)
	}
	if idx >= len(s.nameMap) {
		s.nameMap = append(s.nameMap, make([]nameEntry, idx-len(s.nameMap)+1)...)
	}
	entry := s.nameMap[idx]
	if entry.LocalLen != 0 || entry.NSLen != 0 || entry.Sym != 0 || entry.NS != 0 {
		return entry
	}
	localOff := len(s.nameLocal)
	s.nameLocal = append(s.nameLocal, local...)
	nsOff := len(s.nameNS)
	s.nameNS = append(s.nameNS, nsBytes...)
	nsID := s.namespaceID(nsBytes)
	sym := runtime.SymbolID(0)
	if nsID != 0 {
		sym = s.rt.Symbols.Lookup(nsID, local)
	}
	entry = nameEntry{
		Sym:      sym,
		NS:       nsID,
		LocalOff: uint32(localOff),
		LocalLen: uint32(len(local)),
		NSOff:    uint32(nsOff),
		NSLen:    uint32(len(nsBytes)),
	}
	s.nameMap[idx] = entry
	return entry
}
