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

func (s *Session) newValidation(code xsderrors.ErrorCode, msg, path string, line, column int) xsderrors.Validation {
	return xsderrors.Validation{
		Code:     string(code),
		Message:  msg,
		Document: s.documentURI,
		Path:     path,
		Line:     line,
		Column:   column,
	}
}

func (s *Session) recordValidationError(err error, line, column int) error {
	return s.recordValidationErrorAtPath(err, s.pathString(), line, column)
}

func (s *Session) recordValidationErrors(errs []error, line, column int) error {
	return s.recordValidationErrorsAtPath(errs, s.pathString(), line, column)
}

func (s *Session) recordValidationErrorAtPath(err error, path string, line, column int) error {
	if err == nil {
		return nil
	}
	code, msg, ok := validationErrorInfo(err)
	if !ok {
		return xsderrors.ValidationList{s.newValidation(xsderrors.ErrXMLParse, err.Error(), path, line, column)}
	}
	s.validationErrors = append(s.validationErrors, s.newValidation(code, msg, path, line, column))
	return nil
}

func (s *Session) recordValidationErrorsAtPath(errs []error, path string, line, column int) error {
	if len(errs) == 0 {
		return nil
	}
	for _, err := range errs {
		if fatal := s.recordValidationErrorAtPath(err, path, line, column); fatal != nil {
			return fatal
		}
	}
	return nil
}

func (s *Session) validationList() error {
	if s == nil || len(s.validationErrors) == 0 {
		return nil
	}
	out := make(xsderrors.ValidationList, len(s.validationErrors))
	copy(out, s.validationErrors)
	out.Sort()
	return out
}
