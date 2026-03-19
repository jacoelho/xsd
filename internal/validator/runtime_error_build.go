package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

func newValidationError(code xsderrors.ErrorCode, msg string) error {
	return diag.New(code, msg)
}
