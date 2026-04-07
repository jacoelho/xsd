package semantics

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestValidateSubstitutionGroupFinalChainRestriction(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:sg"
           targetNamespace="urn:sg"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence/>
  </xs:complexType>

  <xs:complexType name="Mid">
    <xs:complexContent>
      <xs:restriction base="tns:Base">
        <xs:sequence/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>

  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Mid">
        <xs:sequence/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>

  <xs:element name="head" type="tns:Base" final="restriction"/>
  <xs:element name="member" type="tns:Derived" substitutionGroup="tns:head"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateReferences(schema)
	if len(errs) == 0 {
		t.Fatalf("expected substitution group final restriction error")
	}
	found := false
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), "final for restriction") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected substitution group final restriction error, got %v", errs[0])
	}
}

func TestValidateSubstitutionGroupFinalExtensionMethodLabel(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:sg"
           targetNamespace="urn:sg"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence/>
  </xs:complexType>

  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>

  <xs:element name="head" type="tns:Base" final="extension"/>
  <xs:element name="member" type="tns:Derived" substitutionGroup="tns:head"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateReferences(schema)
	if len(errs) == 0 {
		t.Fatal("expected substitution group final extension error")
	}

	found := false
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), "final for extension") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected substitution group final extension error, got %v", errs[0])
	}
}

func TestValidateSubstitutionGroupFinalNilDeclsNoPanic(t *testing.T) {
	tests := []struct {
		name   string
		member *model.ElementDecl
		head   *model.ElementDecl
	}{
		{name: "nil both"},
		{name: "nil member", head: &model.ElementDecl{}},
		{name: "nil head", member: &model.ElementDecl{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateSubstitutionGroupFinal(nil, model.QName{}, tt.member, tt.head); err != nil {
				t.Fatalf("validateSubstitutionGroupFinal() error = %v, want nil", err)
			}
		})
	}
}
