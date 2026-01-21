package validator

import (
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestXsiNilInvalidBoolean(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:nil"
           xmlns:tns="urn:nil"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="n" type="xs:string" nillable="true"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<tns:root xmlns:tns="urn:nil" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <tns:n xsi:nil="maybe"/>
</tns:root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype violation, got %v", violations)
	}
}

func TestXsiNilZeroTreatedAsNotNil(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:nil"
           xmlns:tns="urn:nil"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="n" type="xs:string" nillable="true"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<tns:root xmlns:tns="urn:nil" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <tns:n xsi:nil="0">value</tns:n>
</tns:root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}
