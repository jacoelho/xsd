package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
)

func TestChildPresenceRecordedOnStartElementError(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" default="x">
    <xs:complexType>
      <xs:simpleContent>
        <xs:extension base="xs:string"/>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root><child/></root>`
	err := validateRuntimeDoc(t, schemaXML, docXML)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrTextInElementOnly) {
		t.Fatalf("expected ErrTextInElementOnly, got %+v", list)
	}
}

func TestNilledElementReportsNotEmptyWithInvalidChild(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" nillable="true">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"><child/></root>`
	err := validateRuntimeDoc(t, schemaXML, docXML)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrValidateNilledNotEmpty) {
		t.Fatalf("expected ErrValidateNilledNotEmpty, got %+v", list)
	}
}
