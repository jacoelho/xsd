package schemacheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestUnionEnumerationInvalidValueRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test">
  <xs:simpleType name="Small">
    <xs:restriction base="xs:decimal">
      <xs:maxInclusive value="5"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Large">
    <xs:restriction base="xs:decimal">
      <xs:minInclusive value="10"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="U">
    <xs:union memberTypes="tns:Small tns:Large"/>
  </xs:simpleType>
  <xs:simpleType name="UEnum">
    <xs:restriction base="tns:U">
      <xs:enumeration value="7"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if errs := ValidateStructure(schema); len(errs) == 0 {
		t.Fatalf("expected schema validation errors")
	}
}

func TestUnionEnumerationAcceptsMatchingMember(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="UEnum">
    <xs:restriction base="tns:U">
      <xs:enumeration value="abc"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if errs := ValidateStructure(schema); len(errs) != 0 {
		t.Fatalf("expected schema to be valid, got %v", errs)
	}
}

func TestUnionEnumerationRejectsNoMatchingMember(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
  <xs:simpleType name="UEnum">
    <xs:restriction base="tns:U">
      <xs:enumeration value="1.5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if errs := ValidateStructure(schema); len(errs) == 0 {
		t.Fatalf("expected schema validation errors")
	}
}
