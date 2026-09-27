// Package main exposes WASM bindings for browser XSD validation.
package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/internal/format"
	"github.com/jacoelho/xsd/internal/validate"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	maxXMLBytes          int64 = 2 << 20
	maxFormattedXMLBytes       = maxXMLBytes
	maxXSDBytes          int64 = 1 << 20
	maxValidationErrors        = 100
)

type responseStatus string

const (
	statusOK      responseStatus = "ok"
	statusValid   responseStatus = "valid"
	statusInvalid responseStatus = "invalid"
	statusError   responseStatus = "error"
)

type formatResponse struct {
	Status responseStatus `json:"status"`
	XML    string         `json:"xml,omitempty"`
	Error  string         `json:"error,omitempty"`
	Line   int            `json:"line,omitempty"`
	Column int            `json:"column,omitempty"`
}

type validateResponse struct {
	Status responseStatus `json:"status"`
	Error  string         `json:"error,omitempty"`
	Errors []errorOutput  `json:"errors,omitempty"`
}

type errorOutput struct {
	Category string `json:"category,omitempty"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

// validationAdapter owns the worker-local schema cache. A cached engine is
// valid only for the exact admitted XSD spelling used to compile it.
type validationAdapter struct {
	engine  *xsd.Engine
	xsdText string
}

func formatXMLData(input string) formatResponse {
	if input == "" {
		return formatFailure("XML cannot be empty", 1, 1)
	}
	if int64(len(input)) > maxXMLBytes {
		return formatFailure(fmt.Sprintf("XML exceeds %s limit", byteLimit(maxXMLBytes)), 0, 0)
	}

	var out strings.Builder
	err := format.XMLWithOptions(&out, input, format.Options{MaxOutputBytes: maxFormattedXMLBytes})
	if err != nil {
		resp := formatFailure(errorMessage(err), 0, 0)
		if xerr, ok := errors.AsType[*xsderrors.Error](err); ok {
			resp.Line = xerr.Line()
			resp.Column = xerr.Column()
		}
		return resp
	}
	return formatResponse{Status: statusOK, XML: out.String()}
}

func (a *validationAdapter) validateXMLData(xmlText, xsdText string) validateResponse {
	if xmlText == "" {
		return validationFailure("XML cannot be empty")
	}
	if int64(len(xmlText)) > maxXMLBytes {
		return validationFailure(fmt.Sprintf("XML exceeds %s limit", byteLimit(maxXMLBytes)))
	}
	if int64(len(xsdText)) > maxXSDBytes {
		return validationFailure(fmt.Sprintf("XSD exceeds %s limit", byteLimit(maxXSDBytes)))
	}
	engine, compileErr := a.compile(xsdText)
	if compileErr != nil {
		schemaErrors := collectErrors(compileErr, "xsd")
		if xmlErr := validate.CheckXMLWellFormed(strings.NewReader(xmlText), validate.Options{}); xmlErr != nil {
			xmlErrors := collectErrors(xmlErr, "xml")
			return validationError(errors.Join(xmlErr, compileErr), append(xmlErrors, schemaErrors...))
		}
		return validationError(compileErr, schemaErrors)
	}
	err := engine.ValidateWithOptions(strings.NewReader(xmlText), xsd.ValidateOptions{MaxErrors: maxValidationErrors})
	if err != nil {
		diagnostics := collectErrors(err, "xml")
		if isConclusiveValidation(err) {
			return validationInvalid(diagnostics)
		}
		return validationError(err, diagnostics)
	}
	return validateResponse{Status: statusValid}
}

func (a *validationAdapter) compile(xsdText string) (*xsd.Engine, error) {
	if a.engine != nil && a.xsdText == xsdText {
		return a.engine, nil
	}
	// Evict before compiling a changed source. Failed compilations are not
	// negative-cache entries and must leave the adapter empty.
	a.engine = nil
	a.xsdText = ""
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(xsdText)))
	if err != nil {
		return nil, err
	}
	a.engine = engine
	a.xsdText = xsdText
	return engine, nil
}

func formatFailure(message string, line, column int) formatResponse {
	return formatResponse{Status: statusError, Error: message, Line: line, Column: column}
}

func validationFailure(message string) validateResponse {
	return validateResponse{Status: statusError, Error: message}
}

func validationError(err error, diagnostics []errorOutput) validateResponse {
	message := "validation failed without diagnostics"
	if err != nil && err.Error() != "" {
		message = err.Error()
	}
	response := validateResponse{Status: statusError, Error: message}
	if len(diagnostics) != 0 {
		response.Errors = diagnostics
	}
	return response
}

func validationInvalid(diagnostics []errorOutput) validateResponse {
	if len(diagnostics) == 0 {
		return validationFailure("validation failed without diagnostics")
	}
	return validateResponse{Status: statusInvalid, Errors: diagnostics}
}

func collectErrors(err error, source string) []errorOutput {
	if err == nil {
		return nil
	}
	items := xsderrors.Flatten(err)
	out := make([]errorOutput, 0, len(items))
	for _, item := range items {
		out = append(out, errorToOutput(item, source))
	}
	return out
}

func isConclusiveValidation(err error) bool {
	items := xsderrors.Flatten(err)
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		xerr, ok := errors.AsType[*xsderrors.Error](item)
		if !ok || xerr == nil || xerr.Category() != xsderrors.CategoryValidation || xsderrors.IsUnsupported(item) {
			return false
		}
		//nolint:exhaustive // Only conclusive assessment codes qualify; unknown codes fail closed.
		switch xerr.Code() {
		case xsderrors.CodeValidationRoot,
			xsderrors.CodeValidationElement,
			xsderrors.CodeValidationAttribute,
			xsderrors.CodeValidationText,
			xsderrors.CodeValidationType,
			xsderrors.CodeValidationFacet,
			xsderrors.CodeValidationContent,
			xsderrors.CodeValidationNil,
			xsderrors.CodeValidationIdentity:
		default:
			return false
		}
	}
	return true
}

func errorToOutput(err error, source string) errorOutput {
	if xerr, ok := errors.AsType[*xsderrors.Error](err); ok {
		return errorOutput{
			Category: string(xerr.Category()),
			Code:     string(xerr.Code()),
			Source:   source,
			Path:     xerr.Path(),
			Message:  xsdErrorMessage(xerr),
			Line:     xerr.Line(),
			Column:   xerr.Column(),
		}
	}
	return errorOutput{Source: source, Message: err.Error()}
}

func xsdErrorMessage(err *xsderrors.Error) string {
	if err == nil {
		return ""
	}
	msg := err.Message()
	if err.Cause() != nil {
		if msg != "" {
			msg += ": "
		}
		msg += err.Cause().Error()
	}
	if msg == "" {
		return err.Error()
	}
	return msg
}

func errorMessage(err error) string {
	var formatErr *xsderrors.Error
	if errors.As(err, &formatErr) && formatErr.Cause() != nil {
		return formatErr.Cause().Error()
	}
	return err.Error()
}

func byteLimit(n int64) string {
	if n%(1<<20) == 0 {
		return fmt.Sprintf("%d MiB", n/(1<<20))
	}
	if n%(1<<10) == 0 {
		return fmt.Sprintf("%d KiB", n/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}
