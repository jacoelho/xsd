// Package main exposes WASM bindings for browser XSD validation.
package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jacoelho/xsd"
)

const (
	maxXMLBytes          = 2 << 20
	maxFormattedXMLBytes = maxXMLBytes
	maxXSDBytes          = 1 << 20
	maxValidationErrors  = 100
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
	if len(input) > maxXMLBytes {
		return formatResponse{Error: fmt.Sprintf("XML exceeds %s limit", byteLimit(maxXMLBytes))}
	}

	out := limitedBuilder{limit: maxFormattedXMLBytes}
	err := xsd.FormatXML(&out, strings.NewReader(input))
	if err != nil {
		resp := formatResponse{Error: errorMessage(err)}
		var xerr *xsd.XMLFormatError
		if errors.As(err, &xerr) {
			resp.Line = xerr.Line
			resp.Column = xerr.Column
		}
		return resp
	}
	return formatResponse{XML: out.String()}
}

type limitedBuilder struct {
	builder strings.Builder
	limit   int
}

func (w *limitedBuilder) Write(p []byte) (int, error) {
	if w.builder.Len()+len(p) > w.limit {
		return 0, fmt.Errorf("formatted XML exceeds %s limit", byteLimit(w.limit))
	}
	return w.builder.Write(p)
}

func (w *limitedBuilder) String() string {
	return w.builder.String()
}

func validateXMLData(xmlText, xsdText string) validateResponse {
	if xmlText == "" {
		return validateResponse{Error: "XML cannot be empty"}
	}
	if len(xmlText) > maxXMLBytes {
		return validateResponse{Error: fmt.Sprintf("XML exceeds %s limit", byteLimit(maxXMLBytes))}
	}
	if strings.TrimSpace(xsdText) == "" {
		return validateResponse{Error: "XSD is empty"}
	}
	if len(xsdText) > maxXSDBytes {
		return validateResponse{Error: fmt.Sprintf("XSD exceeds %s limit", byteLimit(maxXSDBytes))}
	}

	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(xsdText)))
	if err != nil {
		return validateResponse{Errors: collectErrors(err, "xsd")}
	}
	err = engine.ValidateWithOptions(strings.NewReader(xmlText), xsd.ValidateOptions{MaxErrors: maxValidationErrors})
	if err != nil {
		return validateResponse{Errors: collectErrors(err, "xml")}
	}
	return validateResponse{Valid: true}
}

func collectErrors(err error, source string) []errorOutput {
	if err == nil {
		return nil
	}
	errs, ok := err.(xsd.Errors)
	if !ok {
		return []errorOutput{errorToOutput(err, source)}
	}
	out := make([]errorOutput, 0, len(errs))
	for _, item := range errs {
		out = append(out, errorToOutput(item, source))
	}
	return out
}

func errorToOutput(err error, source string) errorOutput {
	var xerr *xsd.Error
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

func xsdErrorMessage(err *xsd.Error) string {
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
	var formatErr *xsd.XMLFormatError
	if errors.As(err, &formatErr) && formatErr.Err != nil {
		return formatErr.Err.Error()
	}
	return err.Error()
}

func byteLimit(n int) string {
	if n%(1<<20) == 0 {
		return fmt.Sprintf("%d MiB", n/(1<<20))
	}
	if n%(1<<10) == 0 {
		return fmt.Sprintf("%d KiB", n/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}
