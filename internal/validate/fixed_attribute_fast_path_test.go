package validate

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

// Fixed attributes compare raw text only for preserve-whitespace xs:string
// types without ID/IDREF carriers, after full facet validation;
// whitespace-normalizing and ID/IDREF-carrying types compare typed values and
// keep their document-identity effects.
func TestFixedAttributeFastPathBoundaries(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Ref"><xs:restriction base="xs:IDREF"/></xs:simpleType>
  <xs:simpleType name="Bounded"><xs:restriction base="xs:string"><xs:maxLength value="3"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="BoundedR"><xs:restriction base="Bounded"/></xs:simpleType>
  <xs:simpleType name="Collapsed"><xs:restriction base="xs:string"><xs:whiteSpace value="collapse"/></xs:restriction></xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="token" minOccurs="0"><xs:complexType><xs:attribute name="v" type="xs:token" fixed="a b"/></xs:complexType></xs:element>
        <xs:element name="string" minOccurs="0"><xs:complexType><xs:attribute name="v" type="xs:string" fixed="a b"/></xs:complexType></xs:element>
        <xs:element name="ref" minOccurs="0"><xs:complexType><xs:attribute name="v" type="Ref" fixed="x"/></xs:complexType></xs:element>
        <xs:element name="bounded" minOccurs="0"><xs:complexType><xs:attribute name="v" type="Bounded" fixed="ab"/></xs:complexType></xs:element>
        <xs:element name="boundedR" minOccurs="0"><xs:complexType><xs:attribute name="v" type="BoundedR" fixed="ab"/></xs:complexType></xs:element>
        <xs:element name="collapsed" minOccurs="0"><xs:complexType><xs:attribute name="v" type="Collapsed" fixed="a b"/></xs:complexType></xs:element>
        <xs:element name="target" minOccurs="0"><xs:complexType><xs:attribute name="id" type="xs:ID"/></xs:complexType></xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	s, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, body, message string
		code                xsderrors.Code
	}{
		{name: "token collapses whitespace", body: `<token v=" a   b "/>`},
		{name: "token value differs", body: `<token v="a c"/>`, code: xsderrors.CodeValidationAttribute},
		{name: "string preserves whitespace", body: `<string v="a  b"/>`, code: xsderrors.CodeValidationAttribute},
		{name: "string matches", body: `<string v="a b"/>`},
		{name: "dangling IDREF restriction", body: `<ref v="x"/>`, code: xsderrors.CodeValidationType},
		{name: "resolved IDREF restriction", body: `<ref v="x"/><target id="x"/>`},
		{name: "IDREF restriction collapses whitespace", body: `<ref v=" x "/><target id="x"/>`},
		{name: "faceted string matches", body: `<bounded v="ab"/>`},
		{name: "faceted string differs", body: `<bounded v="ac"/>`, code: xsderrors.CodeValidationAttribute},
		{name: "faceted string violates facet", body: `<bounded v="abcd"/>`, code: xsderrors.CodeValidationFacet, message: "expected at most 3"},
		{name: "inherited facet matches", body: `<boundedR v="ab"/>`},
		{name: "inherited facet differs", body: `<boundedR v="ac"/>`, code: xsderrors.CodeValidationAttribute},
		{name: "inherited facet violation", body: `<boundedR v="abcd"/>`, code: xsderrors.CodeValidationFacet, message: "expected at most 3"},
		{name: "collapsing string restriction", body: `<collapsed v=" a   b "/>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := s.Validate(strings.NewReader("<root>" + test.body + "</root>"))
			if test.code == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			expectXSDCode(t, err, test.code)
			if test.message != "" && !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Validate() error = %v, want message containing %q", err, test.message)
			}
		})
	}
}
