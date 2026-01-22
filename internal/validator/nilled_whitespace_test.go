package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestNilledElementRejectsNonXMLWhitespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string" nillable="true"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	doc := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true">&#xA0;</root>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, doc)
	if !hasViolationCode(violations, errors.ErrNilElementNotEmpty) {
		t.Fatalf("expected nil element not empty error, got %v", violations)
	}
}

func TestNilledElementRejectsXMLWhitespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string" nillable="true"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	doc := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"> </root>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, doc)
	if !hasViolationCode(violations, errors.ErrNilElementNotEmpty) {
		t.Fatalf("expected nil element not empty error, got %v", violations)
	}
}
