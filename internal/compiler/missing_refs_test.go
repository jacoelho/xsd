package compiler

import (
	"strings"
	"testing"
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

	docs, err := parseDocumentSet(schemaXML)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if _, err := buildSchemaForTest(docs, BuildConfig{}); err == nil || !strings.Contains(err.Error(), "attributeGroup") {
		t.Fatalf("expected missing attributeGroup ref error, got %v", err)
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

	docs, err := parseDocumentSet(schemaXML)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if _, err := buildSchemaForTest(docs, BuildConfig{}); err == nil || !strings.Contains(err.Error(), "element") {
		t.Fatalf("expected missing substitutionGroup head error, got %v", err)
	}
}
