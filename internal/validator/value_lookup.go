package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validatorMeta(id runtime.ValidatorID) (runtime.ValidatorMeta, error) {
	if s == nil || s.rt == nil {
		return runtime.ValidatorMeta{}, xsderrors.Invalid("runtime schema missing")
	}
	if id == 0 {
		return runtime.ValidatorMeta{}, xsderrors.Invalid("validator missing")
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return runtime.ValidatorMeta{}, xsderrors.Invalidf("validator %d out of range", id)
	}
	return s.rt.Validators.Meta[id], nil
}

func (s *Session) validatorMetaIfPresent(id runtime.ValidatorID) (runtime.ValidatorMeta, bool, error) {
	if s == nil || s.rt == nil || id == 0 {
		return runtime.ValidatorMeta{}, false, nil
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return runtime.ValidatorMeta{}, false, xsderrors.Invalidf("validator %d out of range", id)
	}
	return s.rt.Validators.Meta[id], true, nil
}

func (s *Session) lookupActualUnionValidator(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver) (runtime.ValidatorID, error) {
	var memberMetrics ValueMetrics
	if _, err := s.validateValueCore(id, canonical, resolver, valueOptions{
		ApplyWhitespace:  true,
		RequireCanonical: true,
	}, &memberMetrics); err == nil {
		_, actual := memberMetrics.State.Actual()
		return actual, nil
	}
	return 0, xsderrors.Invalid("union value does not match any member type")
}
