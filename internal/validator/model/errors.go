package model

import (
	"errors"
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
)

// Error is the model-runtime error value used for structured content-model failures.
type Error struct {
	Code     xsderrors.ErrorCode
	Message  string
	Actual   string
	Expected []string
}

func (e Error) Error() string {
	return e.Message
}

// Details holds extracted structured model error metadata.
type Details struct {
	Code     xsderrors.ErrorCode
	Message  string
	Actual   string
	Expected []string
	OK       bool
}

// New creates one structured model validation error.
func New(code xsderrors.ErrorCode, msg string) error {
	return Error{Code: code, Message: msg}
}

// NewWithDetails creates one structured model validation error with expected/actual details.
func NewWithDetails(code xsderrors.ErrorCode, msg, actual string, expected []string) error {
	return Error{
		Code:     code,
		Message:  msg,
		Actual:   actual,
		Expected: slices.Clone(expected),
	}
}

// DetailsOf extracts structured model error details from one error value.
func DetailsOf(err error) Details {
	if err == nil {
		return Details{}
	}
	var modelErr Error
	if errors.As(err, &modelErr) {
		return Details{
			Code:     modelErr.Code,
			Message:  modelErr.Message,
			Actual:   modelErr.Actual,
			Expected: slices.Clone(modelErr.Expected),
			OK:       true,
		}
	}
	return Details{Message: err.Error()}
}

// Info returns just the model validation error code when one is available.
func Info(err error) (xsderrors.ErrorCode, bool) {
	details := DetailsOf(err)
	return details.Code, details.OK
}
