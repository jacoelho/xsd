package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestSimpleContentRestrictionWhiteSpaceFacet(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Collapsed">
    <xs:simpleContent>
      <xs:restriction base="xs:normalizedString">
        <xs:whiteSpace value="collapse"/>
        <xs:enumeration value="a b"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Collapsed"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="urn:test">a   b</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestSimpleContentRestrictionWhiteSpaceFacetMismatch(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Collapsed">
    <xs:simpleContent>
      <xs:restriction base="xs:normalizedString">
        <xs:whiteSpace value="collapse"/>
        <xs:enumeration value="a b"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Collapsed"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="urn:test">a   b c</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if !hasViolationCode(violations, errors.ErrFacetViolation) {
		t.Fatalf("expected code %s, got %v", errors.ErrFacetViolation, violations)
	}
}
