package model

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

// Error is the model-runtime structured validation error envelope.
type Error = diag.Error

// Details holds extracted structured model error metadata.
type Details = diag.Details

// New creates one structured model validation error.
func New(code xsderrors.ErrorCode, msg string) error {
	return diag.New(code, msg)
}

// NewWithDetails creates one structured model validation error with expected/actual details.
func NewWithDetails(code xsderrors.ErrorCode, msg, actual string, expected []string) error {
	return diag.NewWithDetails(code, msg, actual, expected)
}

// DetailsOf extracts structured model error details from one error value.
func DetailsOf(err error) Details {
	return diag.DetailsOf(err)
}

// Info returns just the model validation error code when one is available.
func Info(err error) (xsderrors.ErrorCode, bool) {
	return diag.Info(err)
}
