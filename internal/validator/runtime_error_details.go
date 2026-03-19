package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/model"
)

func validationErrorDetails(err error) validationDetails {
	details := diag.DetailsOf(err)
	if details.OK {
		return validationDetails{
			code:     details.Code,
			msg:      details.Message,
			actual:   details.Actual,
			expected: details.Expected,
			ok:       true,
		}
	}
	modelDetails := model.DetailsOf(err)
	if modelDetails.OK {
		return validationDetails{
			code:     modelDetails.Code,
			msg:      modelDetails.Message,
			actual:   modelDetails.Actual,
			expected: modelDetails.Expected,
			ok:       true,
		}
	}
	return validationDetails{
		code:     details.Code,
		msg:      details.Message,
		actual:   details.Actual,
		expected: details.Expected,
		ok:       details.OK,
	}
}

func validationErrorInfo(err error) (xsderrors.ErrorCode, bool) {
	if code, ok := diag.Info(err); ok {
		return code, true
	}
	return model.Info(err)
}
