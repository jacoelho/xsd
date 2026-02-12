package validator

import (
	"errors"
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
)

func validationErrorDetails(err error) validationDetails {
	if err == nil {
		return validationDetails{}
	}
	var ve validationError
	if errors.As(err, &ve) {
		return validationDetails{
			code:     ve.code,
			msg:      ve.msg,
			actual:   ve.actual,
			expected: slices.Clone(ve.expected),
			ok:       true,
		}
	}
	if kind, ok := valueErrorKindOf(err); ok {
		switch kind {
		case valueErrInvalid:
			return validationDetails{
				code: xsderrors.ErrDatatypeInvalid,
				msg:  err.Error(),
				ok:   true,
			}
		case valueErrFacet:
			return validationDetails{
				code: xsderrors.ErrFacetViolation,
				msg:  err.Error(),
				ok:   true,
			}
		}
	}
	return validationDetails{msg: err.Error()}
}

func validationErrorInfo(err error) (xsderrors.ErrorCode, bool) {
	details := validationErrorDetails(err)
	return details.code, details.ok
}
