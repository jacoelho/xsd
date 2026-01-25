package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestXsiTypeProhibitedAttributesOnUndeclaredElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Restricted">
    <xs:complexContent>
      <xs:restriction base="xs:anyType">
        <xs:attribute name="bad" use="prohibited"/>
        <xs:anyAttribute namespace="##any" processContents="lax"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="lax" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test">
  <child xsi:type="tns:Restricted" bad="x"/>
</root>`

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, parsed))
	violations := validateStream(t, v, document)
	if !hasViolationCode(violations, errors.ErrAttributeProhibited) {
		t.Fatalf("expected %s, got %v", errors.ErrAttributeProhibited, violations)
	}
}
