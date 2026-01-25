package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestProhibitedAttributeWithFixedIsRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" use="prohibited" fixed="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test" a="x"/>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if !hasViolationCode(violations, errors.ErrAttributeProhibited) {
		t.Fatalf("expected code %s, got %v", errors.ErrAttributeProhibited, violations)
	}
}

func TestProhibitedAttributeWithWildcardRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" use="prohibited" fixed="x"/>
      <xs:anyAttribute namespace="##any" processContents="lax"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test" a="x"/>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if !hasViolationCode(violations, errors.ErrAttributeProhibited) {
		t.Fatalf("expected code %s, got %v", errors.ErrAttributeProhibited, violations)
	}
}
