package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validatorMeta(id runtime.ValidatorID) (runtime.ValidatorMeta, error) {
	if s == nil || s.rt == nil {
		return runtime.ValidatorMeta{}, diag.Invalid("runtime schema missing")
	}
	if id == 0 {
		return runtime.ValidatorMeta{}, diag.Invalid("validator missing")
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return runtime.ValidatorMeta{}, diag.Invalidf("validator %d out of range", id)
	}
	return s.rt.Validators.Meta[id], nil
}

func (s *Session) validatorMetaIfPresent(id runtime.ValidatorID) (runtime.ValidatorMeta, bool, error) {
	if s == nil || s.rt == nil || id == 0 {
		return runtime.ValidatorMeta{}, false, nil
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return runtime.ValidatorMeta{}, false, diag.Invalidf("validator %d out of range", id)
	}
	return s.rt.Validators.Meta[id], true, nil
}

func (s *Session) lookupActualUnionValidator(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver) (runtime.ValidatorID, error) {
	if _, memberMetrics, err := s.validateValueInternalWithMetrics(id, canonical, resolver, valruntime.MemberLookupOptions()); err == nil {
		return valruntime.ActualUnionValidator(memberMetrics.ResultState()), nil
	}
	return 0, diag.Invalid("union value does not match any member type")
}
