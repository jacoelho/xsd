package xsd_test

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestDecimalTotalDigitsAppliesToSchemaLiterals(t *testing.T) {
	for _, kind := range []string{"default", "fixed", "enumeration"} {
		for _, test := range []struct {
			lexical string
			valid   bool
		}{
			{lexical: "-000.000", valid: true},
			{lexical: "0.5000", valid: true},
			{lexical: ".05"},
			{lexical: "-0.005"},
		} {
			t.Run(kind+"/"+test.lexical, func(t *testing.T) {
				declaration := fmt.Sprintf(`<xs:element name="root" type="OneDigit" %s="%s"/>`, kind, test.lexical)
				if kind == "enumeration" {
					declaration = fmt.Sprintf(`<xs:simpleType name="Choice"><xs:restriction base="OneDigit"><xs:enumeration value="%s"/></xs:restriction></xs:simpleType>`, test.lexical)
				}
				source := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="OneDigit"><xs:restriction base="xs:decimal"><xs:totalDigits value="1"/></xs:restriction></xs:simpleType>
  ` + declaration + `
</xs:schema>`
				_, err := xsd.Compile(xsd.Bytes("decimal-literal.xsd", []byte(source)))
				if test.valid {
					if err != nil {
						t.Fatalf("Compile %s=%q: %v", kind, test.lexical, err)
					}
					return
				}
				expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaFacet)
			})
		}
	}
}
