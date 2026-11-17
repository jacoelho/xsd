package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseAttributeUseMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    string
		want    attributeUseMode
		wantErr string
	}{
		{name: "optional", mode: "optional", want: attributeUseOptional},
		{name: "required", mode: "required", want: attributeUseRequired},
		{name: "prohibited", mode: "prohibited", want: attributeUseProhibited},
		{name: "invalid", mode: "bad", wantErr: "invalid attribute use bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseAttributeUseMode(tt.mode)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ParseAttributeUseMode() error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("ParseAttributeUseMode() = %d, want %d", got, tt.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseAttributeUseMode() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAttributeUseValueConstraintAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		admission attributeUseValueConstraintAdmission
		wantErr   string
	}{
		{name: "no default"},
		{name: "default on an unfixed declaration", admission: attributeUseValueConstraintAdmission{HasDefault: true}},
		{name: "no default on a fixed declaration", admission: attributeUseValueConstraintAdmission{ReferencedDeclHasFixed: true}},
		{
			name:      "default conflicts with declaration fixed",
			admission: attributeUseValueConstraintAdmission{HasDefault: true, ReferencedDeclHasFixed: true},
			wantErr:   "attribute use default conflicts with fixed attribute declaration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAttributeUseValueConstraintAdmission(tt.admission)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeUseValueConstraintAdmission() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeUseValueConstraintAdmission() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAttributeUseFixedValueAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		admission attributeUseFixedValueAdmission
		wantErr   string
	}{
		{
			name: "no referenced fixed",
			admission: attributeUseFixedValueAdmission{
				Fixed: valueConstraintIdentity(t, "string", "x"),
			},
		},
		{
			name: "matching referenced fixed",
			admission: attributeUseFixedValueAdmission{
				Fixed:               valueConstraintIdentity(t, "integer", "5"),
				ReferencedDeclFixed: valueConstraintIdentity(t, "decimal", "5"),
			},
		},
		{
			name: "conflicting referenced fixed",
			admission: attributeUseFixedValueAdmission{
				Fixed:               valueConstraintIdentity(t, "boolean", "true"),
				ReferencedDeclFixed: valueConstraintIdentity(t, "string", "true"),
			},
			wantErr: "attribute use fixed value conflicts with fixed attribute declaration",
		},
		{
			name: "inherits referenced fixed",
			admission: attributeUseFixedValueAdmission{
				ReferencedDeclFixed: valueConstraintIdentity(t, "string", "x"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAttributeUseFixedValueAdmission(tt.admission)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeUseFixedValueAdmission() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeUseFixedValueAdmission() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultAndFixedTogetherRejectedAtTheirSource(t *testing.T) {
	t.Parallel()
	const both = `default="1" fixed="1"`
	for _, tt := range []struct {
		name, body, wantMsg string
		line, column        int
	}{
		{name: "global element", body: `<xs:element name="e" ` + both + `/>`, wantMsg: "element cannot have both default and fixed", line: 2, column: 1},
		{name: "local element", body: `<xs:complexType name="T"><xs:sequence><xs:element name="e" ` + both + `/></xs:sequence></xs:complexType>`, wantMsg: "element cannot have both default and fixed", line: 2, column: 39},
		{name: "global attribute", body: `<xs:attribute name="g" ` + both + `/>`, wantMsg: "attribute cannot have both default and fixed", line: 2, column: 1},
		{name: "local attribute", body: `<xs:complexType name="T"><xs:attribute name="a" ` + both + `/></xs:complexType>`, wantMsg: "attribute cannot have both default and fixed", line: 2, column: 26},
		{name: "required local attribute", body: `<xs:complexType name="T"><xs:attribute name="a" use="required" ` + both + `/></xs:complexType>`, wantMsg: "attribute cannot have both default and fixed", line: 2, column: 26},
		{name: "attribute reference", body: `<xs:attribute name="g"/><xs:complexType name="T"><xs:attribute ref="g" ` + both + `/></xs:complexType>`, wantMsg: "attribute cannot have both default and fixed", line: 2, column: 50},
		{name: "reference to fixed declaration", body: `<xs:attribute name="g" fixed="1"/><xs:complexType name="T"><xs:attribute ref="g" ` + both + `/></xs:complexType>`, wantMsg: "attribute cannot have both default and fixed", line: 2, column: 60},
		{name: "required reference", body: `<xs:attribute name="g"/><xs:complexType name="T"><xs:attribute ref="g" use="required" ` + both + `/></xs:complexType>`, wantMsg: "attribute cannot have both default and fixed", line: 2, column: 50},
		{name: "prohibited local attribute", body: `<xs:complexType name="T"><xs:attribute name="a" use="prohibited" ` + both + `/></xs:complexType>`, wantMsg: "attribute cannot have both default and fixed", line: 2, column: 26},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schema := "<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n" + tt.body + "\n</xs:schema>"
			_, err := Compile(Options{}, []source.Source{source.Bytes("constraints.xsd", []byte(schema))})
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("Compile() error = %T %[1]v, want structured diagnostic", err)
			}
			if issue.Code() != xsderrors.CodeSchemaInvalidAttribute || issue.Message() != tt.wantMsg || issue.Line() != tt.line || issue.Column() != tt.column {
				t.Fatalf("diagnostic = %s %q at %d:%d, want %s %q at %d:%d", issue.Code(), issue.Message(), issue.Line(), issue.Column(), xsderrors.CodeSchemaInvalidAttribute, tt.wantMsg, tt.line, tt.column)
			}
		})
	}
}

func valueConstraintIdentity(tb testing.TB, typeName, lexical string) constraintIdentity {
	tb.Helper()
	validated := builtinValueForTest(tb, typeName, lexical)
	return constraintIdentity{Lexical: lexical, Canonical: validated.CanonicalText(), Value: validated, Present: true}
}
