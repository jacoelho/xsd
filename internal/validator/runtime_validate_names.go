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

func (s *Session) namespaceID(nsBytes []byte) runtime.NamespaceID {
	if len(nsBytes) == 0 {
		if s.rt != nil {
			return s.rt.PredefNS.Empty
		}
		return 0
	}
	if s.rt == nil {
		return 0
	}
	return s.rt.Namespaces.Lookup(nsBytes)
}

func (s *Session) makeStartAttrs(attrs []xmlstream.ResolvedAttr) []StartAttr {
	if len(attrs) == 0 {
		return nil
	}
	out := s.attrBuf[:0]
	if cap(out) < len(attrs) {
		out = make([]StartAttr, 0, len(attrs))
	}
	for _, attr := range attrs {
		entry := s.internName(attr.NameID, attr.NS, attr.Local)
		local := attr.Local
		nsBytes := attr.NS
		nameCached := false
		if entry.LocalLen != 0 {
			local = s.nameLocal[entry.LocalOff : entry.LocalOff+entry.LocalLen]
			nameCached = true
		}
		if entry.NSLen != 0 {
			nsBytes = s.nameNS[entry.NSOff : entry.NSOff+entry.NSLen]
			nameCached = true
		}
		out = append(out, StartAttr{
			Sym:        entry.Sym,
			NS:         entry.NS,
			NSBytes:    nsBytes,
			Local:      local,
			NameCached: nameCached,
			Value:      attr.Value,
		})
	}
	s.attrBuf = out[:0]
	return out
}

func (s *Session) finalizeIdentity() []error {
	if s == nil {
		return nil
	}
	if len(s.icState.violations) > 0 {
		errs := append([]error(nil), s.icState.violations...)
		s.icState.violations = s.icState.violations[:0]
		return errs
	}
	if pending := s.icState.drainPending(); len(pending) > 0 {
		return pending
	}
	return nil
}
