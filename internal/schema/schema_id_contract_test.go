package schema_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaIDDiagnosticOrder(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body   string
		category     xsderrors.Category
		code         xsderrors.Code
		message      string
		line, column int
		xmlCause     bool
	}{
		{name: "first repeated value wins", body: `  <xs:annotation id="a"/>
  <xs:annotation id="b"/>
  <xs:annotation id="b"/>
  <xs:annotation id="a"/>`, category: xsderrors.CategorySchemaCompile, code: xsderrors.CodeSchemaInvalidAttribute, message: "duplicate schema id b", line: 4, column: 3},
		{name: "normalized IDs collide", body: `  <xs:annotation id="  a&#x9; "/>
  <xs:annotation id="a"/>`, category: xsderrors.CategorySchemaCompile, code: xsderrors.CodeSchemaInvalidAttribute, message: "duplicate schema id a", line: 3, column: 3},
		{name: "later malformed XML wins", body: `  <xs:annotation id="a"/>
  <xs:annotation id="a"/>
  <xs:annotation></xs:element>`, category: xsderrors.CategorySchemaParse, code: xsderrors.CodeSchemaXML, message: "invalid schema XML", line: 4, column: 18, xmlCause: true},
		{name: "later unsupported feature wins", body: `  <xs:annotation id="a"/>
  <xs:annotation id="a"/>
  <xs:assert/>`, category: xsderrors.CategoryUnsupported, code: xsderrors.CodeUnsupportedXSD11, message: "XSD 1.1 feature assert is not supported", line: 4, column: 3},
		{name: "later empty ID wins", body: `  <xs:annotation id="a"/>
  <xs:annotation id="a"/>
  <xs:annotation id=""/>`, category: xsderrors.CategorySchemaCompile, code: xsderrors.CodeSchemaInvalidAttribute, message: "schema id must be NCName", line: 4, column: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + "\n" + test.body + "\n</xs:schema>"
			compiled, err := schema.Compile(schema.Options{}, []source.Source{source.Bytes("ids.xsd", []byte(input))})
			if compiled != nil {
				t.Fatal("failed compilation published a schema")
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("error = %T %[1]v, want structured diagnostic", err)
			}
			if diagnostic.Category() != test.category || diagnostic.Code() != test.code || diagnostic.Message() != test.message || diagnostic.Path() != "ids.xsd" || diagnostic.Line() != test.line || diagnostic.Column() != test.column {
				t.Fatalf("diagnostic = (%s, %s, %q, %q, %d, %d), want (%s, %s, %q, ids.xsd, %d, %d)", diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), test.category, test.code, test.message, test.line, test.column)
			}
			if test.xmlCause {
				cause, ok := errors.AsType[*xmlstream.Error](diagnostic.Cause())
				if !ok || cause.Error() != "end element </xs:element> does not match start element <xs:annotation>" {
					t.Fatalf("XML cause = %T %v", diagnostic.Cause(), diagnostic.Cause())
				}
			} else if diagnostic.Cause() != nil {
				t.Fatalf("cause = %v, want nil", diagnostic.Cause())
			}
		})
	}
}

func TestSchemaIDsAreDocumentLocal(t *testing.T) {
	t.Parallel()
	const first = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" id="same"><xs:element name="first" type="xs:integer"/></xs:schema>`
	const second = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" id="same"><xs:element name="second" type="xs:boolean"/></xs:schema>`
	compiled, err := schema.Compile(schema.Options{}, []source.Source{source.Bytes("first.xsd", []byte(first)), source.Bytes("second.xsd", []byte(second))})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, compiled, `<first>7</first>`)
	mustValidate(t, compiled, `<second>true</second>`)
	mustNotValidate(t, compiled, `<first>no</first>`, xsderrors.CodeValidationFacet)
	mustNotValidate(t, compiled, `<second>7</second>`, xsderrors.CodeValidationFacet)
}
