package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestEmptyAttributeReferenceOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		accepted []string
		rejected string
		code     xsderrors.Code
	}{
		{
			name: "default",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" type="xs:string" default="base"/>
  <xs:element name="r">
    <xs:complexType><xs:attribute ref="a" default=""/><xs:attribute name="probe" type="xs:string"/></xs:complexType>
    <xs:key name="value"><xs:selector xpath="."/><xs:field xpath="@a"/></xs:key>
    <xs:keyref name="reference" refer="value"><xs:selector xpath="."/><xs:field xpath="@probe"/></xs:keyref>
  </xs:element>
</xs:schema>`,
			accepted: []string{`<r probe=""/>`, `<r a="chosen" probe="chosen"/>`},
			rejected: `<r probe="base"/>`,
			code:     xsderrors.CodeValidationIdentity,
		},
		{
			name: "fixed",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" type="xs:string"/>
  <xs:element name="r"><xs:complexType><xs:attribute ref="a" fixed=""/></xs:complexType></xs:element>
</xs:schema>`,
			accepted: []string{`<r/>`, `<r a=""/>`},
			rejected: `<r a="x"/>`,
			code:     xsderrors.CodeValidationAttribute,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			engine, err := xsd.Compile(xsd.Bytes(test.name+".xsd", []byte(test.schema)))
			if err != nil {
				t.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, document := range test.accepted {
				if err := session.Validate(strings.NewReader(document)); err != nil {
					t.Fatalf("accepted document %q: %v", document, err)
				}
			}
			expectCategoryCode(t, session.Validate(strings.NewReader(test.rejected)), xsderrors.CategoryValidation, test.code)
			if err := session.Validate(strings.NewReader(test.accepted[0])); err != nil {
				t.Fatalf("same-session recovery: %v", err)
			}
		})
	}
}
