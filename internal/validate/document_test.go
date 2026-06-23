package validate

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateDocumentComplete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   DocumentCompleteInput
		code xsderrors.Code
		msg  string
	}{
		{
			name: "complete",
			in:   DocumentCompleteInput{SeenRoot: true},
		},
		{
			name: "no root",
			in:   DocumentCompleteInput{},
			code: xsderrors.CodeValidationRoot,
			msg:  "instance document has no root element",
		},
		{
			name: "unclosed element",
			in: DocumentCompleteInput{
				Context:      StartContext{Path: "/root"},
				SeenRoot:     true,
				OpenElements: 1,
			},
			code: xsderrors.CodeValidationXML,
			msg:  "unclosed element",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDocumentComplete(tt.in)
			if tt.msg == "" {
				if err != nil {
					t.Fatalf("ValidateDocumentComplete() error = %v", err)
				}
				return
			}
			requireCode(t, err, tt.code)
			if !strings.Contains(err.Error(), tt.msg) {
				t.Fatalf("ValidateDocumentComplete() error = %v, want %q", err, tt.msg)
			}
		})
	}
}

func TestValidateDocumentElementStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   DocumentElementStartInput
		want bool
	}{
		{
			name: "first root",
			in:   DocumentElementStartInput{},
		},
		{
			name: "child element",
			in:   DocumentElementStartInput{SeenRoot: true, OpenElements: 1},
		},
		{
			name: "multiple roots",
			in: DocumentElementStartInput{
				Context:  StartContext{Path: "/", Line: 2, Column: 3},
				SeenRoot: true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDocumentElementStart(tt.in)
			if !tt.want {
				if err != nil {
					t.Fatalf("ValidateDocumentElementStart() error = %v", err)
				}
				return
			}
			requireCode(t, err, xsderrors.CodeValidationXML)
			if !strings.Contains(err.Error(), "multiple root elements") {
				t.Fatalf("ValidateDocumentElementStart() error = %v", err)
			}
		})
	}
}

func TestValidateEndElement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   EndElementInput
		msg  string
	}{
		{
			name: "matched",
			in: EndElementInput{
				Name:         xml.Name{Local: "root"},
				Expected:     xml.Name{Local: "root"},
				OpenElements: 1,
			},
		},
		{
			name: "unexpected",
			in: EndElementInput{
				Context: StartContext{Path: "/", Line: 4, Column: 5},
			},
			msg: "unexpected end element",
		},
		{
			name: "mismatch",
			in: EndElementInput{
				Name:         xml.Name{Space: "urn:test", Local: "actual"},
				Expected:     xml.Name{Local: "expected"},
				Context:      StartContext{Path: "/expected", Line: 6, Column: 7},
				OpenElements: 1,
			},
			msg: "end element </{urn:test}actual> does not match start element <expected>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEndElement(tt.in)
			if tt.msg == "" {
				if err != nil {
					t.Fatalf("ValidateEndElement() error = %v", err)
				}
				return
			}
			requireCode(t, err, xsderrors.CodeValidationXML)
			if !strings.Contains(err.Error(), tt.msg) {
				t.Fatalf("ValidateEndElement() error = %v, want %q", err, tt.msg)
			}
		})
	}
}

func TestValidateNameResolution(t *testing.T) {
	t.Parallel()

	if err := ValidateNameResolution(StartContext{}, xml.Name{Space: "p", Local: "root"}, true); err != nil {
		t.Fatalf("ValidateNameResolution(resolved) error = %v", err)
	}
	err := ValidateNameResolution(
		StartContext{Path: "/root", Line: 2, Column: 3},
		xml.Name{Space: "p", Local: "child"},
		false,
	)
	requireCode(t, err, xsderrors.CodeValidationXML)
	if !strings.Contains(err.Error(), "unbound namespace prefix p") {
		t.Fatalf("ValidateNameResolution(unresolved) error = %v", err)
	}
}
