package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestSimpleTypeBaseChainValidation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="A">
    <xs:restriction base="xs:integer"/>
  </xs:simpleType>
  <xs:simpleType name="B">
    <xs:restriction base="tns:A"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:B"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="urn:test">abc</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected code %s, got %v", errors.ErrDatatypeInvalid, violations)
	}
}

func TestSimpleTypeBaseChainListItemValidation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="A">
    <xs:restriction base="xs:integer"/>
  </xs:simpleType>
  <xs:simpleType name="B">
    <xs:restriction base="tns:A"/>
  </xs:simpleType>
  <xs:simpleType name="BList">
    <xs:list itemType="tns:B"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:BList"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="urn:test">1 abc</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected code %s, got %v", errors.ErrDatatypeInvalid, violations)
	}
}
