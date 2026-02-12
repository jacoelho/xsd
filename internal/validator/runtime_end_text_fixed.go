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
	if !fixed.present {
		return nil
	}

	matched, err := s.fixedValueMatches(
		result.textValidator,
		fixed.member,
		result.canonText,
		ValueMetrics{keyKind: result.textKeyKind, keyBytes: result.textKeyBytes},
		resolver,
		fixed.value,
		fixed.key,
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
