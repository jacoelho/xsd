package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestStreamListEnumerationWhitespaceCollapse(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="IntList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="IntListEnum">
    <xs:restriction base="IntList">
      <xs:enumeration value="1 2 3"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="IntListEnum"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	validDoc := "<root>1\t2\n3</root>"
	violations, err := v.ValidateStream(strings.NewReader(validDoc))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}

	invalidDoc := "<root>1 2 4</root>"
	violations, err = v.ValidateStream(strings.NewReader(invalidDoc))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations for enumeration mismatch")
	}
}

func TestStreamListTypesRejectEmptyValue(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:list"
           xmlns:tns="urn:list"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="idrefs" type="xs:IDREFS"/>
        <xs:element name="entities" type="xs:ENTITIES"/>
        <xs:element name="nmtokens" type="xs:NMTOKENS"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<tns:root xmlns:tns="urn:list"><tns:idrefs></tns:idrefs><tns:entities></tns:entities><tns:nmtokens></tns:nmtokens></tns:root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype violation, got %v", violations)
	}
}

func TestStreamListDefaultQNameUsesSchemaContext(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:list"
           xmlns:tns="urn:list"
           elementFormDefault="qualified">
  <xs:simpleType name="QNameList">
    <xs:list itemType="xs:QName"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:QNameList" default="tns:val"/>
</xs:schema>`

	docXML := `<root xmlns="urn:list"/>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}
