package resolver

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestValidateSubstitutionGroupImplicitAnyTypeUsesHeadType(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:sg"
           targetNamespace="urn:sg"
           elementFormDefault="qualified">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" substitutionGroup="tns:head"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateReferences(schema)
	if len(errs) > 0 {
		t.Fatalf("expected no substitution group errors, got %v", errs[0])
	}
}

func TestValidateSubstitutionGroupExplicitAnyTypeRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:sg"
           targetNamespace="urn:sg"
           elementFormDefault="qualified">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" type="xs:anyType" substitutionGroup="tns:head"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateReferences(schema)
	if len(errs) == 0 {
		t.Fatalf("expected substitution group derivation error")
	}
	found := false
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), "not derived from substitution group head type") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected substitution group derivation error, got %v", errs[0])
	}
}

func TestValidateSubstitutionGroupAnyTypeHeadAllowsImplicit(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:sg"
           targetNamespace="urn:sg"
           elementFormDefault="qualified">
  <xs:element name="head" type="xs:anyType"/>
  <xs:element name="member" substitutionGroup="tns:head"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateReferences(schema)
	if len(errs) > 0 {
		t.Fatalf("expected no substitution group errors, got %v", errs[0])
	}
}

func TestValidateSubstitutionGroupAnonymousTypesRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:sg"
           targetNamespace="urn:sg"
           elementFormDefault="qualified">
  <xs:element name="head">
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:enumeration value="a"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
  <xs:element name="member" substitutionGroup="tns:head">
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:enumeration value="b"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateReferences(schema)
	if len(errs) == 0 {
		t.Fatalf("expected substitution group derivation error")
	}
	found := false
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), "not derived from substitution group head type") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected substitution group derivation error, got %v", errs[0])
	}
}
