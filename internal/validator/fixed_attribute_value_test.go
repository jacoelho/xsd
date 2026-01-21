package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestAttributeFixedValueNormalization_Decimal(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:decimal" fixed="1.0"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test" a="1.00"/>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if len(violations) > 0 {
		t.Fatalf("expected no violations, got: %v", violations)
	}
}

func TestAttributeFixedValueNormalization_Boolean(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="b" type="xs:boolean" fixed="1"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test" b="true"/>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if len(violations) > 0 {
		t.Fatalf("expected no violations, got: %v", violations)
	}
}

func TestAttributeRefFixedQNameContext(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:attribute name="code" type="xs:QName" fixed="tns:val"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="tns:code"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	docXML := `<root xmlns="urn:test"/>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) > 0 {
		t.Fatalf("expected no violations, got: %v", violations)
	}
}
