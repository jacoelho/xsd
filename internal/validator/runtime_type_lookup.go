package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) element(id runtime.ElemID) (runtime.Element, bool) {
	if id == 0 || int(id) >= len(s.rt.Elements) {
		return runtime.Element{}, false
	}
	return s.rt.Elements[id], true
}

func (s *Session) typeByID(id runtime.TypeID) (runtime.Type, bool) {
	if id == 0 || int(id) >= len(s.rt.Types) {
		return runtime.Type{}, false
	}
	return s.rt.Types[id], true
}
