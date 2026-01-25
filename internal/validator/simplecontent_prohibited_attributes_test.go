package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestSimpleContentRestrictionProhibitedAttributeWithWildcard(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:string"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="tns:Base">
        <xs:attribute name="a" use="prohibited"/>
        <xs:anyAttribute namespace="##any" processContents="lax"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Restricted"/>
</xs:schema>`

	document := `<root xmlns="urn:test" a="x">value</root>`

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

func TestSimpleContentRestrictionProhibitedAttribute(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:string"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="tns:Base">
        <xs:attribute name="a" use="prohibited"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Restricted"/>
</xs:schema>`

	document := `<root xmlns="urn:test" a="x">value</root>`

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
