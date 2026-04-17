package semantics_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

func mustParsedPreparedSchema(t *testing.T, schemaXML string) *parser.Schema {
	t.Helper()
	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return schema
}

func TestDetectTypeCycle(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:type"
           xmlns:tns="urn:type">
  <xs:simpleType name="A">
    <xs:restriction base="tns:B"/>
  </xs:simpleType>
  <xs:simpleType name="B">
    <xs:restriction base="tns:A"/>
  </xs:simpleType>
</xs:schema>`

	sch := mustParsedPreparedSchema(t, schemaXML)
	if err := semantics.DetectCycles(sch); err == nil {
		t.Fatalf("expected type cycle error")
	}
}

func TestDetectGroupCycle(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:group"
           xmlns:tns="urn:group">
  <xs:group name="G1">
    <xs:sequence>
      <xs:group ref="tns:G2"/>
    </xs:sequence>
  </xs:group>
  <xs:group name="G2">
    <xs:sequence>
      <xs:group ref="tns:G1"/>
    </xs:sequence>
  </xs:group>
</xs:schema>`

	sch := mustParsedPreparedSchema(t, schemaXML)
	if err := semantics.DetectCycles(sch); err == nil {
		t.Fatalf("expected group cycle error")
	}
}

func TestDetectAttributeGroupCycle(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attrgroup"
           xmlns:tns="urn:attrgroup">
  <xs:attributeGroup name="AG1">
    <xs:attributeGroup ref="tns:AG2"/>
  </xs:attributeGroup>
  <xs:attributeGroup name="AG2">
    <xs:attributeGroup ref="tns:AG1"/>
  </xs:attributeGroup>
</xs:schema>`

	sch := mustParsedPreparedSchema(t, schemaXML)
	if err := semantics.DetectCycles(sch); err == nil {
		t.Fatalf("expected attributeGroup cycle error")
	}
}

func TestDetectAttributeGroupMissingReference(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attrgroup"
           xmlns:tns="urn:attrgroup">
  <xs:attributeGroup name="AG1">
    <xs:attributeGroup ref="tns:Missing"/>
  </xs:attributeGroup>
</xs:schema>`

	sch := mustParsedPreparedSchema(t, schemaXML)
	if err := semantics.DetectCycles(sch); err == nil {
		t.Fatalf("expected missing attributeGroup reference error")
	}
}

func TestDetectSubstitutionGroupCycle(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:sub"
           xmlns:tns="urn:sub"
           elementFormDefault="qualified">
  <xs:element name="A" type="xs:string" substitutionGroup="tns:B"/>
  <xs:element name="B" type="xs:string" substitutionGroup="tns:A"/>
</xs:schema>`

	sch := mustParsedPreparedSchema(t, schemaXML)
	if err := semantics.DetectCycles(sch); err == nil {
		t.Fatalf("expected substitutionGroup cycle error")
	}
}

func TestDetectSubstitutionGroupMissingHead(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:sub"
           xmlns:tns="urn:sub"
           elementFormDefault="qualified">
  <xs:element name="Member" type="xs:string" substitutionGroup="tns:Missing"/>
</xs:schema>`

	sch := mustParsedPreparedSchema(t, schemaXML)
	if err := semantics.DetectCycles(sch); err == nil {
		t.Fatalf("expected missing substitutionGroup head error")
	}
}
