package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

func validationErrorDetails(err error) validationDetails {
	details := diag.DetailsOf(err)
	return validationDetails{
		code:     details.Code,
		msg:      details.Message,
		actual:   details.Actual,
		expected: details.Expected,
		ok:       details.OK,
	}
}

func validationErrorInfo(err error) (xsderrors.ErrorCode, bool) {
	return diag.Info(err)
}
