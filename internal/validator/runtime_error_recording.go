package validator

import (
	"slices"

	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func (s *Session) recordValidationError(err error, line, column int) error {
	return s.recordValidationErrorAtPath(err, "", line, column)
}

func (s *Session) recordValidationErrors(errs []error, line, column int) error {
	return s.recordValidationErrorsAtPath(errs, "", line, column)
}

func (s *Session) recordValidationErrorAtPath(err error, path string, line, column int) error {
	if err == nil {
		return nil
	}
	details := validationErrorDetails(err)
	if !details.ok {
		return xsderrors.Error{
			Kind:    xsderrors.KindInternal,
			Code:    xsderrors.ErrValidationInternal,
			Message: details.msg,
			Err:     err,
		}
	}
	if path == "" {
		path = s.pathString()
	}
	validation := s.newValidation(details.code, details.msg, path, line, column)
	validation.Actual = details.actual
	validation.Expected = slices.Clone(details.expected)
	s.validationErrors = append(s.validationErrors, validation)
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
