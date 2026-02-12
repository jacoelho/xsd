package validator

import (
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
)

func newValidationError(code xsderrors.ErrorCode, msg string) error {
	return validationError{code: code, msg: msg}
}

func newValidationErrorWithDetails(code xsderrors.ErrorCode, msg, actual string, expected []string) error {
	return validationError{
		code:     code,
		msg:      msg,
		actual:   actual,
		expected: slices.Clone(expected),
	}
}
