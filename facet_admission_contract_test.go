package xsd_test

import (
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestFacetAdmissionDiagnosticOrder(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		message string
		line    int
		column  int
	}{
		{
			name: "available QName restriction evaluates first literal",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Restricted">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="p:item"/>
      <xs:enumeration/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
			message: "invalid facet value p:item",
			line:    4,
			column:  7,
		},
		{
			name: "available QName simple content evaluates first literal",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:simpleContent><xs:extension base="xs:QName"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:enumeration value="p:item"/>
        <xs:enumeration/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`,
			message: "invalid facet value p:item",
			line:    6,
			column:  9,
		},
		{
			name: "unavailable union restriction defers first literal",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:union memberTypes="absent xs:QName"/></xs:simpleType>
  <xs:simpleType name="Restricted">
    <xs:restriction base="Base">
      <xs:enumeration value="p:item"/>
      <xs:enumeration/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
			message: "enumeration missing value",
			line:    6,
			column:  7,
		},
		{
			name: "unavailable union simple content defers first literal",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Union"><xs:union memberTypes="absent xs:QName"/></xs:simpleType>
  <xs:complexType name="Base"><xs:simpleContent><xs:extension base="Union"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:enumeration value="p:item"/>
        <xs:enumeration/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`,
			message: "enumeration missing value",
			line:    8,
			column:  9,
		},
		{
			name: "unavailable list duplicate precedes malformed attributes",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="List"><xs:list itemType="absent"/></xs:simpleType>
  <xs:complexType name="Base"><xs:simpleContent><xs:extension base="List"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:length value="1"/>
        <xs:length value="bad" fixed="bad"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`,
			message: "duplicate length facet",
			line:    8,
			column:  9,
		},
		{
			name: "unavailable list retains whitespace restriction",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="List"><xs:list itemType="absent"/></xs:simpleType>
  <xs:simpleType name="Restricted">
    <xs:restriction base="List">
      <xs:whiteSpace value="preserve"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
			message: "whiteSpace cannot loosen base whiteSpace",
			line:    5,
			column:  7,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := xsd.Compile(xsd.Bytes("facet-admission.xsd", []byte(test.source)))
			diagnostic, ok := err.(*xsderrors.Error) //nolint:errorlint // A wrapper would violate this diagnostic contract.
			if !ok {
				t.Fatalf("Compile() error = %T %v, want direct structured diagnostic", err, err)
			}
			if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaFacet || diagnostic.Cause() != nil {
				t.Fatalf("diagnostic = (%s, %s, %v), want schema_compile, schema.facet, nil cause", diagnostic.Category(), diagnostic.Code(), diagnostic.Cause())
			}
			if diagnostic.Message() != test.message {
				t.Fatalf("Message() = %q, want %q", diagnostic.Message(), test.message)
			}
			if diagnostic.Path() != "facet-admission.xsd" || diagnostic.Line() != test.line || diagnostic.Column() != test.column {
				t.Fatalf("location = %s:%d:%d, want facet-admission.xsd:%d:%d", diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), test.line, test.column)
			}
		})
	}
}
