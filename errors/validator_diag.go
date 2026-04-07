package errors

import (
	"errors"
	"fmt"
	"slices"
)

// Error is the internal validator error value used for structured runtime failures.
type Error struct {
	Code     ErrorCode
	Message  string
	Actual   string
	Expected []string
}

func (e Error) Error() string {
	return e.Message
}

// Details holds extracted validation error metadata.
type Details struct {
	Code     ErrorCode
	Message  string
	Actual   string
	Expected []string
	OK       bool
}

// Issue describes one code/message pair that can be converted into a validator error.
type Issue interface {
	ValidationCode() ErrorCode
	ValidationMessage() string
}

// New creates one structured validation error.
func New(code ErrorCode, msg string) error {
	return Error{Code: code, Message: msg}
}

// NewWithDetails creates one structured validation error with expected/actual details.
func NewWithDetails(code ErrorCode, msg, actual string, expected []string) error {
	return Error{
		Code:     code,
		Message:  msg,
		Actual:   actual,
		Expected: slices.Clone(expected),
	}
}

// Invalid creates one datatype-invalid error.
func Invalid(msg string) error {
	return New(ErrDatatypeInvalid, msg)
}

// Invalidf creates one formatted datatype-invalid error.
func Invalidf(format string, args ...any) error {
	return Invalid(fmt.Sprintf(format, args...))
}

// Facet creates one facet-violation error.
func Facet(msg string) error {
	return New(ErrFacetViolation, msg)
}

// Facetf creates one formatted facet-violation error.
func Facetf(format string, args ...any) error {
	return Facet(fmt.Sprintf(format, args...))
}

// DetailsOf extracts structured validation details from one error value.
func DetailsOf(err error) Details {
	if err == nil {
		return Details{}
	}
	var ve Error
	if errors.As(err, &ve) {
		return Details{
			Code:     ve.Code,
			Message:  ve.Message,
			Actual:   ve.Actual,
			Expected: slices.Clone(ve.Expected),
			OK:       true,
		}
	}
	return Details{Message: err.Error()}
}

// Info returns just the validation error code when one is available.
func Info(err error) (ErrorCode, bool) {
	details := DetailsOf(err)
	return details.Code, details.OK
}

// AppendIssues converts issue values into structured validator errors.
func AppendIssues[T Issue](dst []error, issues []T) []error {
	if len(issues) == 0 {
		return dst
	}
	for _, issue := range issues {
		code := issue.ValidationCode()
		msg := issue.ValidationMessage()
		if code == "" && msg == "" {
			continue
		}
		dst = append(dst, New(code, msg))
	}
	return dst
}
