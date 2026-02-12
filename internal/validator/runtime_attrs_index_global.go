package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) globalAttributeBySymbol(sym runtime.SymbolID) (runtime.AttrID, bool) {
	if sym == 0 {
		return 0, false
	}
	if s.rt == nil || int(sym) >= len(s.rt.GlobalAttributes) {
		return 0, false
	}
	id := s.rt.GlobalAttributes[sym]
	return id, id != 0
}
