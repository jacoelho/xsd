package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestFacetFixednessAdmission(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:string">
      <xs:minLength value="2" fixed="%s"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:minLength value="3"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="r" type="Derived"/>
</xs:schema>`
	t.Run("numeric false permits narrowing", func(t *testing.T) {
		engine, err := xsd.Compile(xsd.Bytes("facet-fixedness.xsd", []byte(fmt.Sprintf(source, "0"))))
		if err != nil {
			t.Fatal(err)
		}
		if err = engine.Validate(strings.NewReader("<r>abc</r>")); err != nil {
			t.Fatalf("valid narrowed value: %v", err)
		}
		err = engine.Validate(strings.NewReader("<r>ab</r>"))
		diagnostic, ok := err.(*xsderrors.Error) //nolint:errorlint // A wrapper would violate this diagnostic contract.
		if !ok {
			t.Fatalf("Validate() error = %T %v, want direct structured diagnostic", err, err)
		}
		if diagnostic.Category() != xsderrors.CategoryValidation || diagnostic.Code() != xsderrors.CodeValidationFacet || diagnostic.Message() != "invalid simple content: length facet failed" || diagnostic.Cause() != nil {
			t.Fatalf("unexpected diagnostic: %#v", diagnostic)
		}
		if diagnostic.Path() != "/r" || diagnostic.Line() != 1 || diagnostic.Column() != 6 {
			t.Fatalf("location = %s:%d:%d, want /r:1:6", diagnostic.Path(), diagnostic.Line(), diagnostic.Column())
		}
	})
	for _, test := range []struct {
		name    string
		source  string
		code    xsderrors.Code
		message string
		line    int
		column  int
	}{
		{
			name: "numeric true prevents narrowing", source: fmt.Sprintf(source, "1"),
			code: xsderrors.CodeSchemaFacet, message: "fixed cardinality facet cannot change", line: 7, column: 3,
		},
		{
			name: "unavailable simple content admits fixed before size",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="List"><xs:list itemType="absent"/></xs:simpleType>
  <xs:complexType name="Base"><xs:simpleContent><xs:extension base="List"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:length value="bad" fixed="bad"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`,
			code: xsderrors.CodeSchemaInvalidAttribute, message: "invalid boolean attribute fixed", line: 7, column: 9,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := xsd.Compile(xsd.Bytes("facet-fixedness.xsd", []byte(test.source)))
			diagnostic, ok := err.(*xsderrors.Error) //nolint:errorlint // A wrapper would violate this diagnostic contract.
			if !ok {
				t.Fatalf("Compile() error = %T %v, want direct structured diagnostic", err, err)
			}
			if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != test.code || diagnostic.Message() != test.message || diagnostic.Cause() != nil {
				t.Fatalf("unexpected diagnostic: %#v", diagnostic)
			}
			if diagnostic.Path() != "facet-fixedness.xsd" || diagnostic.Line() != test.line || diagnostic.Column() != test.column {
				t.Fatalf("location = %s:%d:%d, want facet-fixedness.xsd:%d:%d", diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), test.line, test.column)
			}
		})
	}
}
