package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestStreamValidatorValidSequence(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:int" minOccurs="0"/>
      </xs:sequence>
      <xs:attribute name="id" type="xs:ID"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" id="a1"><a>hi</a><b>1</b></root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}
}

func TestStreamValidatorUnexpectedChild(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test"><c/></root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations")
	}
	if violations[0].Code != string(errors.ErrUnexpectedElement) {
		t.Fatalf("expected code %s, got %s", errors.ErrUnexpectedElement, violations[0].Code)
	}
}

func TestStreamValidatorWildcardSkip(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip"/>
        <xs:element name="tail" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" xmlns:other="urn:other">
  <other:skip><bad/></other:skip>
  <tail>ok</tail>
</root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}
}

func TestStreamValidatorNilledElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="n" type="xs:string" nillable="true"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <n xsi:nil="true">text</n>
</root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations")
	}
	if violations[0].Code != string(errors.ErrNilElementNotEmpty) {
		t.Fatalf("expected code %s, got %s", errors.ErrNilElementNotEmpty, violations[0].Code)
	}
}

func validateStreamDoc(t *testing.T, schemaXML, docXML string) ([]errors.Validation, error) {
	t.Helper()

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}
	v := New(mustCompile(t, schema))
	return v.ValidateStream(strings.NewReader(docXML))
}
