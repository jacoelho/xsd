package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

// An XML declaration is document text, so its diagnostics embed at most a
// bounded, escaped prefix of the hostile attribute values.
func TestXMLDeclarationDiagnosticsBoundHostileText(t *testing.T) {
	t.Parallel()

	// 128-byte value bound, escaped up to 4x, plus fixed message text.
	const maxMessage = 1024
	const schemaText = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("s.xsd", []byte(schemaText)))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		decl     string
		instance xsderrors.Code
		schema   xsderrors.Code
		contains string
	}{
		{"hostile version", `version="1.` + strings.Repeat("0", 1<<20) + `"`, xsderrors.CodeUnsupportedXMLVersion, xsderrors.CodeUnsupportedXMLVersion, ""},
		{"hostile encoding", `version="1.0" encoding="A` + strings.Repeat("!", 1<<20) + `"`, xsderrors.CodeValidationXML, xsderrors.CodeSchemaXML, ""},
		{"hostile invalid version", `version="1!` + strings.Repeat("!", 1<<20) + `"`, xsderrors.CodeValidationXML, xsderrors.CodeSchemaXML, ""},
		{"short invalid version", `version="1!"`, xsderrors.CodeValidationXML, xsderrors.CodeSchemaXML, `invalid XML declaration version "1!"`},
		{"short invalid encoding", `version="1.0" encoding="A!"`, xsderrors.CodeValidationXML, xsderrors.CodeSchemaXML, `invalid XML declaration encoding "A!"`},
		{"short unsupported version", `version="1.1"`, xsderrors.CodeUnsupportedXMLVersion, xsderrors.CodeUnsupportedXMLVersion, `XML version 1.1 is not supported`},
		{"C1 control in version", "version=\"1\u0085\"", xsderrors.CodeValidationXML, xsderrors.CodeSchemaXML, `invalid XML declaration version`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := func(t *testing.T, err error, want xsderrors.Code) {
				t.Helper()
				var diagnostic *xsderrors.Error
				if !errors.As(err, &diagnostic) {
					t.Fatalf("error %T is not a diagnostic: %v", err, err)
				}
				if diagnostic.Code() != want {
					t.Errorf("code = %s, want %s", diagnostic.Code(), want)
				}
				if len(err.Error()) >= maxMessage {
					t.Errorf("Error() is %d bytes, want < %d", len(err.Error()), maxMessage)
				}
				if tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
					t.Errorf("Error() = %q, want it to contain %q", err.Error(), tt.contains)
				}
				if strings.ContainsRune(err.Error(), '\u0085') {
					t.Errorf("Error() contains a raw C1 control")
				}
			}
			t.Run("instance", func(t *testing.T) {
				err := engine.Validate(strings.NewReader(`<?xml ` + tt.decl + `?><r/>`))
				check(t, err, tt.instance)
			})
			t.Run("schema", func(t *testing.T) {
				_, err := xsd.Compile(xsd.Bytes("h.xsd", []byte(`<?xml `+tt.decl+`?>`+schemaText)))
				check(t, err, tt.schema)
			})
		})
	}
}
