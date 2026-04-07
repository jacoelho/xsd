package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
)

func newValidationError(code xsderrors.ErrorCode, msg string) error {
	return xsderrors.New(code, msg)
}
