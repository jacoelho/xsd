package validator

import (
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestDerivedSimpleTypeValidatesPrimitiveLexical(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		doc    string
	}{
		{
			name: "date",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="A">
    <xs:restriction base="xs:date"/>
  </xs:simpleType>
  <xs:simpleType name="B">
    <xs:restriction base="tns:A"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:B"/>
</xs:schema>`,
			doc: `<root xmlns="urn:test">not-a-date</root>`,
		},
		{
			name: "decimal",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="A">
    <xs:restriction base="xs:decimal"/>
  </xs:simpleType>
  <xs:simpleType name="B">
    <xs:restriction base="tns:A"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:B"/>
</xs:schema>`,
			doc: `<root xmlns="urn:test">not-a-decimal</root>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, err := validateStreamDoc(t, tt.schema, tt.doc)
			if err != nil {
				t.Fatalf("ValidateStream() error = %v", err)
			}
			if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
				t.Fatalf("expected datatype violation, got %v", violations)
			}
		})
	}
}
