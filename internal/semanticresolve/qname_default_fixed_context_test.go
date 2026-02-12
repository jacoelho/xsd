package semanticresolve

import (
	"strings"
	"testing"

	parser "github.com/jacoelho/xsd/internal/parser"
)

func TestResolveUnionDefaultQNameMissingPrefix(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:simpleType name="UnionType">
    <xs:union memberTypes="xs:QName xs:int"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:UnionType" default="p:val"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	res := NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	if errs := ValidateReferences(schema); len(errs) == 0 {
		t.Fatalf("expected reference errors for undefined QName prefix in default")
	}
}

func TestResolveListFixedQNameMissingPrefix(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:simpleType name="QNameList">
    <xs:list itemType="xs:QName"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:QNameList" fixed="p:val"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	res := NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	if errs := ValidateReferences(schema); len(errs) == 0 {
		t.Fatalf("expected reference errors for undefined QName prefix in fixed list")
	}
}
