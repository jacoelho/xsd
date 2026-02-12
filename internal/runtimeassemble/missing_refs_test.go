package runtimeassemble

import (
	"strings"
	"testing"

	parser "github.com/jacoelho/xsd/internal/parser"
	model "github.com/jacoelho/xsd/internal/types"
)

func TestBuildSchemaMissingAttributeGroupRefFails(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="T">
    <xs:attributeGroup ref="tns:MissingGroup"/>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if _, err := buildSchemaForTest(sch, BuildConfig{}); err == nil || !strings.Contains(err.Error(), "attribute group") {
		t.Fatalf("expected missing attributeGroup ref error, got %v", err)
	}
}

func TestBuildSchemaAllGroupSubstitutionMemberMissingID(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" substitutionGroup="tns:head" type="xs:string"/>
  <xs:complexType name="T">
    <xs:all>
      <xs:element ref="tns:head"/>
    </xs:all>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`

	sch, err := resolveSchema(schemaXML)
	if err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	memberQName := model.QName{Namespace: "urn:test", Local: "member"}
	filtered := sch.GlobalDecls[:0]
	for _, decl := range sch.GlobalDecls {
		if decl.Kind == parser.GlobalDeclElement && decl.Name == memberQName {
			continue
		}
		filtered = append(filtered, decl)
	}
	sch.GlobalDecls = filtered

	if _, err := buildSchemaForTest(sch, BuildConfig{}); err == nil || !strings.Contains(err.Error(), "all group substitution element") {
		t.Fatalf("expected missing substitution member ID error, got %v", err)
	}
}

func TestBuildSchemaMissingSubstitutionGroupHeadFails(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="member" substitutionGroup="tns:missing" type="xs:string"/>
</xs:schema>`

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if _, err := buildSchemaForTest(sch, BuildConfig{}); err == nil || !strings.Contains(err.Error(), "substitutionGroup") {
		t.Fatalf("expected missing substitutionGroup head error, got %v", err)
	}
}
