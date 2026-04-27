package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) stringKind(meta runtime.ValidatorMeta) (runtime.StringKind, bool) {
	validators := s.rt.ValidatorBundle()
	if int(meta.Index) >= len(validators.String) {
		return runtime.StringAny, false
	}
	return validators.String[meta.Index].Kind, true
}

func (s *Session) integerKind(meta runtime.ValidatorMeta) (runtime.IntegerKind, bool) {
	validators := s.rt.ValidatorBundle()
	if int(meta.Index) >= len(validators.Integer) {
		return runtime.IntegerAny, false
	}
	return validators.Integer[meta.Index].Kind, true
}
