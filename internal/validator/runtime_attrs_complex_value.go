package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

type attrValidationSpec struct {
	validator   runtime.ValidatorID
	fixedMember runtime.ValidatorID
	fixed       runtime.ValueRef
	fixedKey    runtime.ValueKeyRef
}

func (s *Session) validateComplexAttrValue(
	validated []StartAttr,
	attr StartAttr,
	resolver value.NSResolver,
	storeAttrs bool,
	spec attrValidationSpec,
	seenID *bool,
) ([]StartAttr, error) {
	canon, metrics, err := s.validateValueInternalWithMetrics(spec.validator, attr.Value, resolver, valueOptions{
		applyWhitespace:  true,
		trackIDs:         true,
		requireCanonical: spec.fixed.Present,
		storeValue:       storeAttrs,
		needKey:          spec.fixed.Present,
	})
	if err != nil {
		return nil, wrapValueError(err)
	}
	if s.isIDValidator(spec.validator) {
		if *seenID {
			return nil, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
		}
		*seenID = true
	}
	validated = s.appendValidatedAttr(validated, attr, storeAttrs, canon, metrics.keyKind, metrics.keyBytes)
	if spec.fixed.Present {
		match, err := s.fixedValueMatches(spec.validator, spec.fixedMember, canon, metrics, resolver, spec.fixed, spec.fixedKey)
		if err != nil {
			return nil, err
		}
		if !match {
			return nil, newValidationError(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
		}
	}
	return validated, nil
}
