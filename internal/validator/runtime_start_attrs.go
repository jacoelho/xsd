package validator

import (
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) makeStartAttrs(resolvedAttrs []xmlstream.ResolvedAttr) []attrs.Start {
	if len(resolvedAttrs) == 0 {
		return nil
	}
	out := s.attrState.Starts[:0]
	if cap(out) < len(resolvedAttrs) {
		out = make([]attrs.Start, 0, len(resolvedAttrs))
	}
	for _, attr := range resolvedAttrs {
		entry := s.internName(attr.NameID, attr.NS, attr.Local)
		local := attr.Local
		nsBytes := attr.NS
		nameCached := false
		storedNS, storedLocal := s.Names.EntryBytes(entry)
		if entry.LocalLen != 0 {
			local = storedLocal
			nameCached = true
		}
		if entry.NSLen != 0 {
			nsBytes = storedNS
			nameCached = true
		}
		out = append(out, attrs.Start{
			Sym:        entry.Sym,
			NS:         entry.NS,
			NSBytes:    nsBytes,
			Local:      local,
			NameCached: nameCached,
			Value:      attr.Value,
		})
	}
	s.attrState.Starts = out[:0]
	return out
}
