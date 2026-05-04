package xsd

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorCategory identifies the validation phase that produced an error.
type ErrorCategory string

// Error categories returned by Error.Category.
const (
	SchemaParseErrorCategory   ErrorCategory = "schema_parse"
	SchemaCompileErrorCategory ErrorCategory = "schema_compile"
	UnsupportedErrorCategory   ErrorCategory = "unsupported"
	ValidationErrorCategory    ErrorCategory = "validation"
	InternalErrorCategory      ErrorCategory = "internal"
)

// ErrorCode is a stable machine-readable error code.
type ErrorCode string

// Error codes returned by Error.Code.
const (
	ErrSchemaRead             ErrorCode = "schema.read"
	ErrSchemaXML              ErrorCode = "schema.xml"
	ErrSchemaRoot             ErrorCode = "schema.root"
	ErrSchemaDuplicate        ErrorCode = "schema.duplicate"
	ErrSchemaReference        ErrorCode = "schema.reference"
	ErrSchemaFacet            ErrorCode = "schema.facet"
	ErrSchemaOccurrence       ErrorCode = "schema.occurrence"
	ErrSchemaContentModel     ErrorCode = "schema.content_model"
	ErrSchemaNoSources        ErrorCode = "schema.no_sources"
	ErrSchemaMissingSource    ErrorCode = "schema.missing_source"
	ErrSchemaInvalidAttribute ErrorCode = "schema.invalid_attribute"
	ErrSchemaIdentity         ErrorCode = "schema.identity"
	ErrSchemaLimit            ErrorCode = "schema.limit"
	ErrUnsupportedDTD         ErrorCode = "unsupported.dtd"
	ErrUnsupportedExternal    ErrorCode = "unsupported.external_entity"
	ErrUnsupportedEntity      ErrorCode = "unsupported.entity"
	ErrUnsupportedNonUTF8     ErrorCode = "unsupported.non_utf8"
	ErrUnsupportedRedefine    ErrorCode = "unsupported.xs_redefine"
	ErrUnsupportedRegex       ErrorCode = "unsupported.regex"
	ErrUnsupportedDateTime    ErrorCode = "unsupported.datetime_range"
	ErrUnsupportedSchemaHint  ErrorCode = "unsupported.xsi_schema_location"
	ErrUnsupportedXML11       ErrorCode = "unsupported.xml_1_1"
	ErrUnsupportedXSD11       ErrorCode = "unsupported.xsd_1_1"
	ErrValidationXML          ErrorCode = "validation.xml"
	ErrValidationRoot         ErrorCode = "validation.root"
	ErrValidationElement      ErrorCode = "validation.element"
	ErrValidationAttribute    ErrorCode = "validation.attribute"
	ErrValidationText         ErrorCode = "validation.text"
	ErrValidationType         ErrorCode = "validation.type"
	ErrValidationFacet        ErrorCode = "validation.facet"
	ErrValidationContent      ErrorCode = "validation.content"
	ErrValidationNil          ErrorCode = "validation.nil"
	ErrValidationIdentity     ErrorCode = "validation.identity"
	ErrInternalInvariant      ErrorCode = "internal.invariant"
)

// Error is the structured error type returned by this package.
type Error struct {
	Err      error
	Category ErrorCategory
	Code     ErrorCode
	Path     string
	Message  string
	Line     int
	Column   int
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	var b strings.Builder
	if e.Code != "" {
		b.WriteString(string(e.Code))
	} else {
		b.WriteString(string(e.Category))
	}
	if e.Line > 0 {
		fmt.Fprintf(&b, " at %d:%d", e.Line, e.Column)
	}
	if e.Path != "" {
		b.WriteString(" ")
		b.WriteString(e.Path)
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if e.Err != nil {
		if e.Message == "" {
			b.WriteString(": ")
		} else {
			b.WriteString(": ")
		}
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsUnsupported reports whether err represents an unsupported feature.
func IsUnsupported(err error) bool {
	x, ok := errors.AsType[*Error](err)
	return ok && x.Category == UnsupportedErrorCategory
}

func schemaParse(code ErrorCode, line, col int, msg string, err error) error {
	return &Error{Category: SchemaParseErrorCategory, Code: code, Line: line, Column: col, Message: msg, Err: err}
}

func schemaCompile(code ErrorCode, msg string) error {
	return &Error{Category: SchemaCompileErrorCategory, Code: code, Message: msg}
}

func unsupported(code ErrorCode, msg string) error {
	return &Error{Category: UnsupportedErrorCategory, Code: code, Message: msg}
}

func validation(code ErrorCode, line, col int, path, msg string) error {
	return &Error{Category: ValidationErrorCategory, Code: code, Line: line, Column: col, Path: path, Message: msg}
}
