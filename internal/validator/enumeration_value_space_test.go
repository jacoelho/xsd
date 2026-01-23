package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestEnumerationValueSpaceDecimal(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="DecEnum">
    <xs:restriction base="xs:decimal">
      <xs:enumeration value="1.0"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="DecEnum"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	okDoc := "<root>1.00</root>"
	if violations := validateStream(t, v, okDoc); len(violations) > 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}

	badDoc := "<root>1.10</root>"
	if violations := validateStream(t, v, badDoc); len(violations) == 0 {
		t.Fatalf("expected enumeration violation")
	}
}

func TestEnumerationValueSpaceBoolean(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="BoolEnum">
    <xs:restriction base="xs:boolean">
      <xs:enumeration value="true"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="BoolEnum"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	okDoc := "<root>1</root>"
	if violations := validateStream(t, v, okDoc); len(violations) > 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}

	badDoc := "<root>false</root>"
	if violations := validateStream(t, v, badDoc); len(violations) == 0 {
		t.Fatalf("expected enumeration violation")
	}
}

func TestEnumerationValueSpaceUnionOverlappingMembers(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           xmlns:tns="http://example.com/test"
           elementFormDefault="qualified">
  <xs:simpleType name="UnionBase">
    <xs:union memberTypes="xs:integer xs:decimal"/>
  </xs:simpleType>
  <xs:simpleType name="UnionEnum">
    <xs:restriction base="tns:UnionBase">
      <xs:enumeration value="1.0"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:UnionEnum"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	okDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">1</root>`
	if violations := validateStream(t, v, okDoc); len(violations) > 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}

	badDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">2</root>`
	if violations := validateStream(t, v, badDoc); len(violations) == 0 {
		t.Fatalf("expected enumeration violation")
	}
}
