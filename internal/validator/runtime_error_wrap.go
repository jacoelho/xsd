package validator

import (
	"errors"
	"slices"
)

func wrapValueError(err error) error {
	if err == nil {
		return nil
	}
	var ve validationError
	if errors.As(err, &ve) {
		return err
	}
	details := validationErrorDetails(err)
	if details.ok {
		return validationError{
			code:     details.code,
			msg:      details.msg,
			actual:   details.actual,
			expected: slices.Clone(details.expected),
		}
	}
	return err
}
