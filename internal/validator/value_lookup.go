package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func (s *Session) validatorMeta(id runtime.ValidatorID) (runtime.ValidatorMeta, error) {
	if s == nil || s.rt == nil {
		return runtime.ValidatorMeta{}, xsderrors.Invalid("runtime schema missing")
	}
	if id == 0 {
		return runtime.ValidatorMeta{}, xsderrors.Invalid("validator missing")
	}
	meta, ok := s.rt.ValidatorMeta(id)
	if !ok {
		return runtime.ValidatorMeta{}, xsderrors.Invalidf("validator %d out of range", id)
	}
	return meta, nil
}

func (s *Session) validatorMetaIfPresent(id runtime.ValidatorID) (runtime.ValidatorMeta, bool, error) {
	if s == nil || s.rt == nil || id == 0 {
		return runtime.ValidatorMeta{}, false, nil
	}
	meta, ok := s.rt.ValidatorMeta(id)
	if !ok {
		return runtime.ValidatorMeta{}, false, xsderrors.Invalidf("validator %d out of range", id)
	}
	return meta, true, nil
}

func (s *Session) lookupActualUnionValidator(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver) (runtime.ValidatorID, error) {
	result, err := s.validateValue(valueRequest{
		Validator: id,
		Lexical:   canonical,
		Resolver:  resolver,
		Options: valueOptions{
			ApplyWhitespace:  true,
			RequireCanonical: true,
		},
	})
	if err == nil {
		return result.ActualValidator, nil
	}
	return 0, xsderrors.Invalid("union value does not match any member type")
}
