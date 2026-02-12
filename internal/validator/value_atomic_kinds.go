package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) stringKind(meta runtime.ValidatorMeta) (runtime.StringKind, bool) {
	if int(meta.Index) >= len(s.rt.Validators.String) {
		return runtime.StringAny, false
	}
	return s.rt.Validators.String[meta.Index].Kind, true
}

func (s *Session) integerKind(meta runtime.ValidatorMeta) (runtime.IntegerKind, bool) {
	if int(meta.Index) >= len(s.rt.Validators.Integer) {
		return runtime.IntegerAny, false
	}
	return s.rt.Validators.Integer[meta.Index].Kind, true
}
