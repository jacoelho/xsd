package validator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestQNameFixedValueElementPrefixAgnostic(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:t"
           xmlns:p="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:QName" fixed="p:val"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	xmlOK := `<?xml version="1.0"?><root xmlns="urn:t" xmlns:q="urn:t">q:val</root>`
	violations := validateStream(t, v, xmlOK)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}

	xmlBad := `<?xml version="1.0"?><root xmlns="urn:t" xmlns:q="urn:other">q:val</root>`
	violations = validateStream(t, v, xmlBad)
	if !hasViolationCode(violations, errors.ErrElementFixedValue) {
		t.Fatalf("expected fixed value violation, got %v", violations)
	}
}

func TestQNameFixedValueAttributePrefixAgnostic(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:t"
           xmlns:p="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="attr" type="xs:QName" fixed="p:val"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	xmlOK := `<?xml version="1.0"?><root xmlns="urn:t" xmlns:q="urn:t" attr="q:val"/>`
	violations := validateStream(t, v, xmlOK)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}

	xmlBad := `<?xml version="1.0"?><root xmlns="urn:t" xmlns:q="urn:other" attr="q:val"/>`
	violations = validateStream(t, v, xmlBad)
	if !hasViolationCode(violations, errors.ErrAttributeFixedValue) {
		t.Fatalf("expected fixed value violation, got %v", violations)
	}
}

func TestNotationFixedValueElementPrefixAgnostic(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:t"
           xmlns:p="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:notation name="note" public="pub"/>
  <xs:element name="root" type="xs:NOTATION" fixed="p:note"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	xmlOK := fmt.Sprintf(`<?xml version="1.0"?><root xmlns="urn:t" xmlns:q="urn:t">q:%s</root>`, "note")
	violations := validateStream(t, v, xmlOK)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestQNameDefaultValueUsesSchemaContext(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:t"
           xmlns:p="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:QName" default="p:val"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	xmlOK := `<?xml version="1.0"?><root xmlns="urn:t"/>`
	violations := validateStream(t, v, xmlOK)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestQNameFixedValueAttributeMissingUsesSchemaContext(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:t"
           xmlns:p="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="attr" type="xs:QName" fixed="p:val"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	xmlOK := `<?xml version="1.0"?><root xmlns="urn:t"/>`
	violations := validateStream(t, v, xmlOK)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}
