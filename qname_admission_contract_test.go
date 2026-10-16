package xsd_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestQNameAdmissionPreservesErrorPriorityAndEmptyValues(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body, message string
	}{
		{"element reference", `<xs:complexType name="container"><xs:sequence>
  <xs:element type="t:T" substitutionGroup="s:H" ref="r:R"/>
</xs:sequence></xs:complexType>`, "unbound QName prefix r"},
		{"element substitution", `<xs:complexType name="container"><xs:sequence>
  <xs:element type="t:T" substitutionGroup="s:H" ref="xs:string"/>
</xs:sequence></xs:complexType>`, "unbound QName prefix s"},
		{"attribute reference", `<xs:complexType name="container">
  <xs:attribute type="t:T" ref="r:R"/>
</xs:complexType>`, "unbound QName prefix r"},
		{"empty type", `
  <xs:element name="value" type=""/>`, "invalid QName "},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + test.body + `</xs:schema>`
			engine, err := xsd.Compile(xsd.Bytes(test.name+".xsd", []byte(source)))
			if engine != nil {
				t.Fatal("failed compilation returned an engine")
			}
			expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaReference)
			if diagnostics := xsderrors.Flatten(err); len(diagnostics) != 1 {
				t.Fatalf("diagnostics = %v; want one", diagnostics)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic == nil {
				t.Fatalf("missing structured diagnostic: %v", err)
			}
			if diagnostic.Message() != test.message || diagnostic.Path() != test.name+".xsd" ||
				diagnostic.Line() != 2 || diagnostic.Column() != 3 || diagnostic.Cause() != nil {
				t.Fatalf("diagnostic = %v; want %q at %s.xsd:2:3 with nil cause", diagnostic, test.message, test.name)
			}
		})
	}
}
