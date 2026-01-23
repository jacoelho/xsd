package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestUnionEnumerationWhitespaceCollapse(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="UnionBase">
    <xs:union memberTypes="xs:string xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="UnionWithEnum">
    <xs:restriction base="tns:UnionBase">
      <xs:enumeration value="a b"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:UnionWithEnum"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	xmlOK := `<?xml version="1.0"?><root xmlns="urn:test">a   b</root>`
	violations := validateStream(t, v, xmlOK)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestUnionPatternWhitespaceCollapse(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="UnionBase">
    <xs:union memberTypes="xs:string xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="UnionWithPattern">
    <xs:restriction base="tns:UnionBase">
      <xs:pattern value="a b"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:UnionWithPattern"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	xmlOK := `<?xml version="1.0"?><root xmlns="urn:test">a   b</root>`
	violations := validateStream(t, v, xmlOK)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}
