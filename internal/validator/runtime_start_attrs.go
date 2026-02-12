package validator

import "github.com/jacoelho/xsd/pkg/xmlstream"

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
