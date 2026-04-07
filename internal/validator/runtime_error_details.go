package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
)

func validationErrorDetails(err error) validationDetails {
	details := xsderrors.DetailsOf(err)
	return validationDetails{
		code:     details.Code,
		msg:      details.Message,
		actual:   details.Actual,
		expected: details.Expected,
		ok:       details.OK,
	}
}

func validationErrorInfo(err error) (xsderrors.ErrorCode, bool) {
	return xsderrors.Info(err)
}
