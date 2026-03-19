package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) validateEndTextFixed(
	result endTextState,
	hasContent bool,
	elem runtime.Element,
	elemOK bool,
	ct runtime.ComplexType,
	hasComplexText bool,
	resolver sessionResolver,
	path *string,
) []error {
	if result.canonText == nil || !hasContent {
		return nil
	}

	fixed := selectTextFixedConstraint(elem, elemOK, ct, hasComplexText)
	if !fixed.Present {
		return nil
	}

	matched, err := matchFixedValue(
		result.textValidator,
		fixed.Member,
		result.canonText,
		result.textKeyKind,
		result.textKeyBytes,
		result.textKeyKind != runtime.VKInvalid,
		fixed.Value,
		fixed.Key,
		func(ref runtime.ValueRef) []byte { return valueBytes(s.rt.Values, ref) },
		func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
			return s.keyForCanonicalValue(validator, canonical, resolver, member)
		},
	)
	if err != nil {
		s.ensurePath(path)
		return []error{err}
	}
	if !matched {
		s.ensurePath(path)
		return []error{newValidationError(xsderrors.ErrElementFixedValue, "fixed element value mismatch")}
	}
	return nil
}
