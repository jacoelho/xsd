package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) internSparseName(id NameID, nsBytes, local []byte) nameEntry {
	if s == nil {
		return nameEntry{}
	}
	if s.nameMapSparse == nil {
		s.nameMapSparse = make(map[NameID]nameEntry)
	}
	if entry, ok := s.nameMapSparse[id]; ok {
		return entry
	}
	if len(s.nameMapSparse) >= maxNameMapSize {
		nsID := s.namespaceID(nsBytes)
		sym := runtime.SymbolID(0)
		if nsID != 0 {
			sym = s.rt.Symbols.Lookup(nsID, local)
		}
		return nameEntry{Sym: sym, NS: nsID}
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
	entry := nameEntry{
		Sym:      sym,
		NS:       nsID,
		LocalOff: uint32(localOff),
		LocalLen: uint32(len(local)),
		NSOff:    uint32(nsOff),
		NSLen:    uint32(len(nsBytes)),
	}
	s.nameMapSparse[id] = entry
	return entry
}
