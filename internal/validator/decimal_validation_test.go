package validator

import (
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestValidateDecimalRejectsFraction(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:decimal"
           xmlns:tns="urn:decimal"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:decimal"/>
</xs:schema>`

	docXML := `<tns:root xmlns:tns="urn:decimal">1/2</tns:root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype violation, got %v", violations)
	}
}
