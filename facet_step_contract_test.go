package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestFacetStepEmptyEnumeration(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{
			name: "simple restriction",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Empty">
    <xs:restriction base="xs:string">
      <xs:enumeration value=""/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="r" type="Empty"/>
</xs:schema>`,
		},
		{
			name: "simple content restriction",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Empty">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:enumeration value=""/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="r" type="Empty"/>
</xs:schema>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, err := xsd.Compile(xsd.Bytes("empty-enumeration.xsd", []byte(test.source)))
			if err != nil {
				t.Fatalf("Compile(): %v", err)
			}
			for _, document := range []string{"<r/>", "<r></r>"} {
				if validationErr := engine.Validate(strings.NewReader(document)); validationErr != nil {
					t.Fatalf("Validate(%q): %v", document, validationErr)
				}
			}
			err = engine.Validate(strings.NewReader("<r>x</r>"))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
		})
	}
}

func TestFacetStepDiagnosticLocations(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		message string
		line    int
		column  int
	}{
		{
			name: "simple restriction lower pair precedes upper pair",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Limited">
    <xs:restriction base="xs:decimal">
      <xs:pattern value="[0-9]+"/>
      <xs:maxInclusive value="10"/>
      <xs:maxExclusive value="9"/>
      <xs:minInclusive value="0"/>
      <xs:minExclusive value="1"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
			message: "minInclusive and minExclusive cannot both be specified",
			line:    3,
			column:  5,
		},
		{
			name: "simple content upper pair uses first facet location",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:simpleContent><xs:extension base="xs:decimal"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Limited">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:pattern value="[0-9]+"/>
        <xs:maxInclusive value="10"/>
        <xs:maxExclusive value="9"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`,
			message: "maxInclusive and maxExclusive cannot both be specified",
			line:    6,
			column:  9,
		},
		{
			name: "simple restriction late facet keeps child location",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Limited">
    <xs:restriction base="xs:decimal">
      <xs:pattern value="[0-9]+"/>
      <xs:enumeration value="2"/>
      <xs:enumeration/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
			message: "enumeration missing value",
			line:    6,
			column:  7,
		},
		{
			name: "simple content late facet keeps child location",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:simpleContent><xs:extension base="xs:decimal"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Limited">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:pattern value="[0-9]+"/>
        <xs:enumeration value="2"/>
        <xs:enumeration/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`,
			message: "enumeration missing value",
			line:    8,
			column:  9,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := xsd.Compile(xsd.Bytes("facet-step.xsd", []byte(test.source)))
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
			if diagnostic.Path() != "facet-step.xsd" || diagnostic.Line() != test.line || diagnostic.Column() != test.column {
				t.Fatalf("location = %s:%d:%d, want facet-step.xsd:%d:%d", diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), test.line, test.column)
			}
		})
	}
}
