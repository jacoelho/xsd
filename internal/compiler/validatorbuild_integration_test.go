package compiler

import (
	"testing"

	"github.com/jacoelho/xsd/internal/schemair"
)

func TestSchemaIRPreservesEffectiveSemantics(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:plan"
           targetNamespace="urn:plan"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="baseChild" type="xs:string"/>
    </xs:sequence>
    <xs:attribute name="baseAttr" type="xs:string" use="required"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="tail" type="xs:int"/>
        </xs:sequence>
        <xs:attribute name="extra" type="xs:int"/>
        <xs:anyAttribute processContents="lax"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="MeasureType">
    <xs:simpleContent>
      <xs:extension base="xs:double"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="LengthType">
    <xs:simpleContent>
      <xs:restriction base="tns:MeasureType"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	docs, err := parseDocumentSet(schemaXML)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	prepared, err := Prepare(docs)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	derived := mustIRTypeID(t, prepared, "urn:plan", "Derived")
	derivedPlan := mustIRComplexPlan(t, prepared, derived)
	if derivedPlan.Particle == 0 {
		t.Fatal("Derived particle = 0")
	}
	if len(derivedPlan.Attrs) != 2 {
		t.Fatalf("Derived attributes = %d, want 2", len(derivedPlan.Attrs))
	}
	first := prepared.ir.AttributeUses[derivedPlan.Attrs[0]-1].Name.Local
	second := prepared.ir.AttributeUses[derivedPlan.Attrs[1]-1].Name.Local
	if first != "baseAttr" || second != "extra" {
		t.Fatalf("Derived attribute order = [%s %s], want [baseAttr extra]", first, second)
	}
	if derivedPlan.AnyAttr == 0 {
		t.Fatal("Derived wildcard = nil, want extension anyAttribute")
	}

	length := mustIRTypeID(t, prepared, "urn:plan", "LengthType")
	rt, err := prepared.Build(BuildConfig{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	runtimeTypeID := int(len(prepared.ir.BuiltinTypes) + int(length))
	if runtimeTypeID <= 0 || runtimeTypeID >= len(rt.TypeTable()) {
		t.Fatalf("LengthType runtime type ID %d out of range", runtimeTypeID)
	}
	complexID := rt.TypeTable()[runtimeTypeID].Complex.ID
	if complexID == 0 || int(complexID) >= len(rt.ComplexTypeTable()) {
		t.Fatalf("LengthType complex ID %d out of range", complexID)
	}
	if rt.ComplexTypeTable()[complexID].TextValidator == 0 {
		t.Fatal("LengthType text validator = 0")
	}
}

func mustIRTypeID(t *testing.T, prepared *Prepared, namespace, local string) uint32 {
	t.Helper()
	for _, typ := range prepared.ir.Types {
		if typ.Name.Namespace == namespace && typ.Name.Local == local {
			return uint32(typ.ID)
		}
	}
	t.Fatalf("missing IR type {%s}%s", namespace, local)
	return 0
}

func mustIRComplexPlan(t *testing.T, prepared *Prepared, id uint32) schemair.ComplexTypePlan {
	t.Helper()
	for _, plan := range prepared.ir.ComplexTypes {
		if uint32(plan.TypeDecl) == id {
			return plan
		}
	}
	t.Fatalf("missing complex plan %d", id)
	return schemair.ComplexTypePlan{}
}
