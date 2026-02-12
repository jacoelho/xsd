package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) expectedGlobalElements() []string {
	if s == nil || s.rt == nil {
		return nil
	}
	names := make([]string, 0, len(s.rt.GlobalElements))
	for sym, elem := range s.rt.GlobalElements {
		if sym == 0 || elem == 0 {
			continue
		}
		name := s.elementName(elem)
		if name == "" {
			name = s.symbolName(runtime.SymbolID(sym))
		}
		names = append(names, name)
	}
	return normalizeExpectedElements(names)
}
