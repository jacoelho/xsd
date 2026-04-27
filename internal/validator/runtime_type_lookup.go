package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) element(id runtime.ElemID) (*runtime.Element, bool) {
	if s == nil || s.rt == nil {
		return nil, false
	}
	return s.rt.ElementRef(id)
}

func (s *Session) typeByID(id runtime.TypeID) (runtime.Type, bool) {
	if s == nil || s.rt == nil {
		return runtime.Type{}, false
	}
	return s.rt.Type(id)
}
