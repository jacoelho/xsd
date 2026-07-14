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

type formatResponse struct {
	XML    string `json:"xml,omitempty"`
	Error  string `json:"error,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

type validateResponse struct {
	Error  string        `json:"error,omitempty"`
	Errors []errorOutput `json:"errors,omitempty"`
	Valid  bool          `json:"valid"`
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

func formatXMLData(input string) formatResponse {
	if input == "" {
		return formatResponse{Error: "XML cannot be empty", Line: 1, Column: 1}
	}
	if int64(len(input)) > maxXMLBytes {
		return formatResponse{Error: fmt.Sprintf("XML exceeds %s limit", byteLimit(maxXMLBytes))}
	}

	var out strings.Builder
	err := format.XMLWithOptions(&out, strings.NewReader(input), format.Options{MaxOutputBytes: maxFormattedXMLBytes})
	if err != nil {
		resp := formatResponse{Error: errorMessage(err)}
		var xerr *xsderrors.Error
		if errors.As(err, &xerr) {
			resp.Line = xerr.Line
			resp.Column = xerr.Column
		}
		return resp
	}
	return formatResponse{XML: out.String()}
}

func validateXMLData(xmlText, xsdText string) validateResponse {
	if xmlText == "" {
		return validateResponse{Error: "XML cannot be empty"}
	}
	if int64(len(xmlText)) > maxXMLBytes {
		return validateResponse{Error: fmt.Sprintf("XML exceeds %s limit", byteLimit(maxXMLBytes))}
	}
	if int64(len(xsdText)) > maxXSDBytes {
		return validateResponse{Error: fmt.Sprintf("XSD exceeds %s limit", byteLimit(maxXSDBytes))}
	}
	engine, compileErr := xsd.Compile(xsd.Bytes("schema.xsd", []byte(xsdText)))
	if compileErr != nil {
		schemaErrors := collectErrors(compileErr, "xsd")
		if xmlErr := validate.CheckXMLWellFormed(strings.NewReader(xmlText), validate.Options{}); xmlErr != nil {
			xmlErrors := collectErrors(xmlErr, "xml")
			return validateResponse{Errors: append(xmlErrors, schemaErrors...)}
		}
		return validateResponse{Errors: schemaErrors}
	}
	err := engine.ValidateWithOptions(strings.NewReader(xmlText), xsd.ValidateOptions{MaxErrors: maxValidationErrors})
	if err != nil {
		return validateResponse{Errors: collectErrors(err, "xml")}
	}
	return validateResponse{Valid: true}
}

func collectErrors(err error, source string) []errorOutput {
	if err == nil {
		return nil
	}
	var errs xsderrors.Errors
	if !errors.As(err, &errs) {
		return []errorOutput{errorToOutput(err, source)}
	}
	out := make([]errorOutput, 0, len(errs))
	for _, item := range errs {
		out = append(out, errorToOutput(item, source))
	}
	return out
}

func errorToOutput(err error, source string) errorOutput {
	var xerr *xsderrors.Error
	if errors.As(err, &xerr) {
		return errorOutput{
			Category: string(xerr.Category),
			Code:     string(xerr.Code),
			Source:   source,
			Path:     xerr.Path,
			Message:  xsdErrorMessage(xerr),
			Line:     xerr.Line,
			Column:   xerr.Column,
		}
	}
	return errorOutput{Source: source, Message: err.Error()}
}

func xsdErrorMessage(err *xsderrors.Error) string {
	if err == nil {
		return ""
	}
	msg := err.Message
	if err.Err != nil {
		if msg != "" {
			msg += ": "
		}
		msg += err.Err.Error()
	}
	if msg == "" {
		return err.Error()
	}
	return msg
}

func errorMessage(err error) string {
	var formatErr *xsderrors.Error
	if errors.As(err, &formatErr) && formatErr.Err != nil {
		return formatErr.Err.Error()
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
