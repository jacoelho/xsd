// Package xsderrors defines structured XSD diagnostics.
package xsderrors

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const nilErrorString = "<nil>"

// ErrSchemaNotFound reports that a resolver could not resolve a schema.
var ErrSchemaNotFound = errors.New("schema not found")

// Category identifies the operation class that produced an error.
type Category string

// Error categories.
const (
	CategorySchemaParse   Category = "schema_parse"
	CategorySchemaCompile Category = "schema_compile"
	CategoryUnsupported   Category = "unsupported"
	CategoryValidation    Category = "validation"
	CategoryCanceled      Category = "canceled"
	CategoryInternal      Category = "internal"
)

// Code is a stable machine-readable error code.
type Code string

// Error codes.
const (
	CodeSchemaRead             Code = "schema.read"
	CodeSchemaXML              Code = "schema.xml"
	CodeSchemaRoot             Code = "schema.root"
	CodeSchemaDuplicate        Code = "schema.duplicate"
	CodeSchemaReference        Code = "schema.reference"
	CodeSchemaFacet            Code = "schema.facet"
	CodeSchemaOccurrence       Code = "schema.occurrence"
	CodeSchemaContentModel     Code = "schema.content_model"
	CodeSchemaNoSources        Code = "schema.no_sources"
	CodeSchemaInvalidAttribute Code = "schema.invalid_attribute"
	CodeSchemaIdentity         Code = "schema.identity"
	CodeSchemaLimit            Code = "schema.limit"
	CodeCompileCanceled        Code = "compile.canceled"
	CodeUnsupportedDTD         Code = "unsupported.dtd"
	CodeUnsupportedExternal    Code = "unsupported.external_entity"
	CodeUnsupportedEntity      Code = "unsupported.entity"
	CodeUnsupportedNonUTF8     Code = "unsupported.non_utf8"
	CodeUnsupportedRedefine    Code = "unsupported.xs_redefine"
	CodeUnsupportedRegex       Code = "unsupported.regex"
	CodeUnsupportedSchemaHint  Code = "unsupported.xsi_schema_location"
	CodeUnsupportedXML11       Code = "unsupported.xml_1_1"
	CodeUnsupportedXSD11       Code = "unsupported.xsd_1_1"
	CodeValidationXML          Code = "validation.xml"
	CodeValidationRoot         Code = "validation.root"
	CodeValidationElement      Code = "validation.element"
	CodeValidationAttribute    Code = "validation.attribute"
	CodeValidationText         Code = "validation.text"
	CodeValidationType         Code = "validation.type"
	CodeValidationFacet        Code = "validation.facet"
	CodeValidationContent      Code = "validation.content"
	CodeValidationNil          Code = "validation.nil"
	CodeValidationIdentity     Code = "validation.identity"
	CodeValidationOption       Code = "validation.option"
	CodeValidationSession      Code = "validation.session"
	CodeValidationLimit        Code = "validation.limit"
	CodeValidationCanceled     Code = "validation.canceled"
	CodeInternalInvariant      Code = "internal.invariant"
)

// Error is the public structured diagnostic returned by compile and validation
// operations.
type Error struct {
	Err      error
	Category Category
	Code     Code
	Path     string
	Message  string
	Line     int
	Column   int
}

// Errors is returned when validation finds multiple recoverable errors.
type Errors []error

func (e *Error) Error() string {
	if e == nil {
		return nilErrorString
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
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

func (e Errors) Error() string {
	var first error
	count := 0
	for _, err := range e {
		if err == nil {
			continue
		}
		if first == nil {
			first = err
		}
		count++
	}
	switch count {
	case 0:
		return nilErrorString
	case 1:
		return first.Error()
	default:
		return fmt.Sprintf("%d validation errors: %s", count, first)
	}
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e Errors) Unwrap() []error {
	for i, err := range e {
		if err != nil {
			continue
		}
		children := make([]error, 0, len(e)-1)
		children = append(children, e[:i]...)
		for _, child := range e[i+1:] {
			if child != nil {
				children = append(children, child)
			}
		}
		return children
	}
	return []error(e)
}

// IsUnsupported reports whether err represents an unsupported feature.
func IsUnsupported(err error) bool {
	switch x := any(err).(type) {
	case nil:
		return false
	case *Error:
		if x == nil {
			return false
		}
		return x.Category == CategoryUnsupported || IsUnsupported(x.Err)
	case Errors:
		return slices.ContainsFunc(x, IsUnsupported)
	}
	if x, ok := errors.AsType[*Error](err); ok && x != nil && (x.Category == CategoryUnsupported || IsUnsupported(x.Err)) {
		return true
	}
	if x, ok := err.(interface{ Unwrap() []error }); ok {
		return slices.ContainsFunc(x.Unwrap(), IsUnsupported)
	}
	if x, ok := err.(interface{ Unwrap() error }); ok {
		return IsUnsupported(x.Unwrap())
	}
	return false
}

// SchemaParse returns a schema parsing diagnostic.
func SchemaParse(code Code, line, col int, msg string, err error) error {
	return &Error{Category: CategorySchemaParse, Code: code, Line: line, Column: col, Message: msg, Err: err}
}

// SchemaCompile returns a schema compilation diagnostic.
func SchemaCompile(code Code, msg string) error {
	return &Error{Category: CategorySchemaCompile, Code: code, Message: msg}
}

// SchemaCompileAt returns a schema compilation diagnostic with source location.
func SchemaCompileAt(path string, line, col int, code Code, msg string) error {
	return &Error{Category: CategorySchemaCompile, Code: code, Path: path, Line: line, Column: col, Message: msg}
}

// WithSchemaCompileLocation attaches a source location to a compile diagnostic
// when it has none.
func WithSchemaCompileLocation(path string, line, col int, err error) error {
	if err == nil {
		return nil
	}
	x, ok := directDiagnostic(err)
	if !ok || x.Category != CategorySchemaCompile || x.Line > 0 || x.Path != "" {
		return err
	}
	y := *x
	y.Path, y.Line, y.Column = path, line, col
	return &y
}

// WithPath attaches path to a structured diagnostic when it has none.
func WithPath(path string, err error) error {
	if path == "" || err == nil {
		return err
	}
	x, ok := directDiagnostic(err)
	if !ok || x.Path != "" {
		return err
	}
	y := *x
	y.Path = path
	return &y
}

func directDiagnostic(err error) (*Error, bool) {
	// Decoration is intentionally restricted to a top-level diagnostic. Traversing
	// wrappers or aggregates would discard their error-tree structure when cloning.
	x, ok := err.(*Error) //nolint:errorlint // Exact top-level matching is required here.
	return x, ok && x != nil
}

// Unsupported returns an unsupported-feature diagnostic.
func Unsupported(code Code, msg string) error {
	return &Error{Category: CategoryUnsupported, Code: code, Message: msg}
}

// UnsupportedAt returns an unsupported-feature diagnostic with location data.
func UnsupportedAt(code Code, line, col int, path, msg string, err error) error {
	return &Error{Category: CategoryUnsupported, Code: code, Line: line, Column: col, Path: path, Message: msg, Err: err}
}

// Validation returns a document validation diagnostic.
func Validation(code Code, line, col int, path, msg string) error {
	return &Error{Category: CategoryValidation, Code: code, Line: line, Column: col, Path: path, Message: msg}
}

// Canceled returns a structured cancellation diagnostic that preserves cause.
func Canceled(code Code, msg string, cause error) error {
	return &Error{Category: CategoryCanceled, Code: code, Message: msg, Err: cause}
}

// InternalInvariant returns an internal invariant diagnostic.
func InternalInvariant(msg string) error {
	return &Error{Category: CategoryInternal, Code: CodeInternalInvariant, Message: msg}
}
