package validatorgen

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
)

func TestCompileBuildsComplexTypePlan(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:plan"
           xmlns:tns="urn:plan"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:attribute name="a" type="xs:string"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:any namespace="##any" processContents="skip" minOccurs="0"/>
        </xs:sequence>
        <xs:anyAttribute namespace="##other" processContents="lax"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	sch, reg, err := parseAndAssign(schemaXML)
	if err != nil {
		t.Fatalf("parseAndAssign() error = %v", err)
	}
	compiled, err := Compile(sch, reg)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if compiled.ComplexTypes == nil {
		t.Fatal("compiled.ComplexTypes is nil")
	}

	derivedName := model.QName{Namespace: "urn:plan", Local: "Derived"}
	derivedType, ok := sch.TypeDefs[derivedName]
	if !ok {
		t.Fatalf("missing type %s", derivedName)
	}
	derivedCT, ok := model.AsComplexType(derivedType)
	if !ok || derivedCT == nil {
		t.Fatalf("type %s is not complex", derivedName)
	}
	attrs, wildcard, ok := compiled.ComplexTypes.AttributeUses(derivedCT)
	if !ok {
		t.Fatalf("complex plan missing entry for %s", derivedName)
	}
	if len(attrs) == 0 {
		t.Fatalf("plan attrs for %s are empty", derivedName)
	}
	if wildcard == nil {
		t.Fatalf("plan wildcard for %s is nil", derivedName)
	}
	if particle, ok := compiled.ComplexTypes.Content(derivedCT); !ok || particle == nil {
		t.Fatalf("plan content particle missing for %s", derivedName)
	}
}

func TestCompileBuildsComplexTypePlan_AllRegistryComplexTypesPresent(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:plan"
           xmlns:tns="urn:plan"
           elementFormDefault="qualified">
  <xs:complexType name="A"/>
  <xs:complexType name="B">
    <xs:simpleContent>
      <xs:extension base="xs:string"/>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`

	sch, reg, err := parseAndAssign(schemaXML)
	if err != nil {
		t.Fatalf("parseAndAssign() error = %v", err)
	}
	compiled, err := Compile(sch, reg)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if compiled.ComplexTypes == nil {
		t.Fatal("compiled.ComplexTypes is nil")
	}
	for _, entry := range reg.TypeOrder {
		ct, ok := model.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		if _, ok := compiled.ComplexTypes.Entry(ct); !ok {
			t.Fatalf("complex plan missing type entry %s (id=%d)", entry.QName, schema.TypeID(entry.ID))
		}
	}
}
