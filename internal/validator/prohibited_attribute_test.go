package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestProhibitedAttributeRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:complexContent>
      <xs:restriction base="tns:Base">
        <xs:attribute name="a" use="prohibited"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Restricted"/>
</xs:schema>`

	docXML := `<root xmlns="urn:test" a="x"/>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if !hasViolationCode(violations, errors.ErrAttributeProhibited) {
		t.Fatalf("expected prohibited attribute violation, got %v", violations)
	}
}

func TestProhibitedAttributeFixedAllowedAtParse(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string" use="prohibited" fixed="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	_, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
}
