package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestWildcardLaxIgnoresUnrelatedLocalElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="TypeA">
    <xs:sequence>
      <xs:any processContents="lax" maxOccurs="unbounded"/>
    </xs:sequence>
  </xs:complexType>

  <xs:complexType name="TypeB">
    <xs:sequence>
      <xs:element name="item" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>

  <xs:element name="root" type="tns:TypeA"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<root xmlns="urn:test"><item>not-int</item></root>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if len(violations) != 0 {
		t.Fatalf("expected no violations for lax wildcard, got %v", violations)
	}
}

func TestWildcardStrictIgnoresUnrelatedLocalElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="TypeA">
    <xs:sequence>
      <xs:any processContents="strict" maxOccurs="unbounded"/>
    </xs:sequence>
  </xs:complexType>

  <xs:complexType name="TypeB">
    <xs:sequence>
      <xs:element name="item" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>

  <xs:element name="root" type="tns:TypeA"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<root xmlns="urn:test"><item>1</item></root>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if !hasViolationCode(violations, errors.ErrWildcardNotDeclared) {
		t.Fatalf("expected wildcard not declared error, got %v", violations)
	}
}
