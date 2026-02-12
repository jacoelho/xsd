package validator

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

const maxExpectedElements = 32

func normalizeExpectedElements(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	filtered := names[:0]
	for _, name := range names {
		if name == "" {
			continue
		}
		filtered = append(filtered, name)
	}
	if len(filtered) == 0 {
		return nil
	}
	slices.Sort(filtered)
	filtered = slices.Compact(filtered)
	if len(filtered) > maxExpectedElements {
		filtered = filtered[:maxExpectedElements]
	}
	return slices.Clone(filtered)
}

func (s *Session) actualElementName(sym runtime.SymbolID, nsID runtime.NamespaceID) string {
	if sym != 0 {
		return s.symbolName(sym)
	}
	if nsID != 0 {
		return "{?}?"
	}
	return ""
}

func (s *Session) symbolName(sym runtime.SymbolID) string {
	if s == nil || s.rt == nil || sym == 0 {
		return ""
	}
	local := s.rt.Symbols.LocalBytes(sym)
	if len(local) == 0 {
		return ""
	}
	nsID := s.rt.Symbols.NS[sym]
	ns := s.rt.Namespaces.Bytes(nsID)
	if len(ns) == 0 {
		return string(local)
	}
	return "{" + string(ns) + "}" + string(local)
}

func (s *Session) elementName(elem runtime.ElemID) string {
	if s == nil || s.rt == nil || elem == 0 {
		return ""
	}
	if int(elem) >= len(s.rt.Elements) {
		return ""
	}
	return s.symbolName(s.rt.Elements[elem].Name)
}
