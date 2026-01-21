package validator

import (
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestInvalidIDREFDoesNotTrack(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item">
          <xs:complexType>
            <xs:attribute name="ref" type="xs:IDREF"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test"><item ref="1abc"/></root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype invalid error, got %v", violations)
	}
	if hasViolationCode(violations, errors.ErrIDRefNotFound) {
		t.Fatalf("unexpected idref-not-found error for invalid value, got %v", violations)
	}
}

func TestInvalidIDDoesNotProduceDuplicate(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test"><item id="1abc"/><item id="1abc"/></root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype invalid error, got %v", violations)
	}
	if hasViolationCode(violations, errors.ErrDuplicateID) {
		t.Fatalf("unexpected duplicate ID error for invalid values, got %v", violations)
	}
}
