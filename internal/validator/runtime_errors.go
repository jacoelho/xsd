package validator

import (
	"errors"

	xsderrors "github.com/jacoelho/xsd/errors"
)

type validationError struct {
	code xsderrors.ErrorCode
	msg  string
}

func (e validationError) Error() string {
	return e.msg
}

func newValidationError(code xsderrors.ErrorCode, msg string) error {
	return validationError{code: code, msg: msg}
}

func validationErrorInfo(err error) (xsderrors.ErrorCode, string, bool) {
	if err == nil {
		return "", "", false
	}
	var ve validationError
	if errors.As(err, &ve) {
		return ve.code, ve.msg, true
	}
	if kind, ok := valueErrorKindOf(err); ok {
		switch kind {
		case valueErrInvalid:
			return xsderrors.ErrDatatypeInvalid, err.Error(), true
		case valueErrFacet:
			return xsderrors.ErrFacetViolation, err.Error(), true
		}
	}
	return "", err.Error(), false
}

func wrapValueError(err error) error {
	if err == nil {
		return nil
	}
	var ve validationError
	if errors.As(err, &ve) {
		return err
	}
	if code, msg, ok := validationErrorInfo(err); ok {
		return validationError{code: code, msg: msg}
	}
	return err
}

func (s *Session) wrapValidationError(err error, line, column int) error {
	if err == nil {
		return nil
	}
	code, msg, ok := validationErrorInfo(err)
	if !ok {
		return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrXMLParse, err.Error(), s.pathString())}
	}
	violation := xsderrors.Validation{
		Code:    string(code),
		Message: msg,
		Path:    s.pathString(),
		Line:    line,
		Column:  column,
	}
	return xsderrors.ValidationList{violation}
}
