package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestMissingRequiredAttributeDiagnosticQuotesNameOnce(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t">
  <xs:attribute name="b"/>
  <xs:element name="r"><xs:complexType>
    <xs:attribute name="a" use="required"/>
    <xs:attribute ref="t:b" use="required"/>
  </xs:complexType></xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("required.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, document, message string }{
		{"unqualified", `<t:r xmlns:t="urn:t" t:b="1"/>`, `missing required attribute "a"`},
		{"qualified", `<t:r xmlns:t="urn:t" a="1"/>`, `missing required attribute "{urn:t}b"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(test.document))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationAttribute)
			if got := err.Error(); !strings.HasSuffix(got, test.message) {
				t.Fatalf("diagnostic = %q, want suffix %q", got, test.message)
			}
		})
	}
}
