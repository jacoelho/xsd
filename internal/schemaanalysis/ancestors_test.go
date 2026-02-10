package schemaanalysis_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
)

func TestBuildAncestorsMasks(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:anc"
           xmlns:tns="urn:anc">
  <xs:complexType name="Base"/>
  <xs:complexType name="Ext">
    <xs:complexContent>
      <xs:extension base="tns:Base"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="Restrict">
    <xs:complexContent>
      <xs:restriction base="tns:Ext"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	registry, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}

	ancestors, err := schema.BuildAncestors(sch, registry)
	if err != nil {
		t.Fatalf("BuildAncestors error = %v", err)
	}

	baseQName := model.QName{Namespace: "urn:anc", Local: "Base"}
	extQName := model.QName{Namespace: "urn:anc", Local: "Ext"}
	restQName := model.QName{Namespace: "urn:anc", Local: "Restrict"}

	baseID := registry.Types[baseQName]
	extID := registry.Types[extQName]
	restID := registry.Types[restQName]

	extOff := ancestors.Offsets[extID]
	extLen := ancestors.Lengths[extID]
	if extLen != 1 {
		t.Fatalf("Ext ancestors length = %d, want 1", extLen)
	}
	if got := ancestors.IDs[extOff]; got != baseID {
		t.Fatalf("Ext ancestor ID = %d, want %d", got, baseID)
	}
	if got := ancestors.Masks[extOff]; got != model.DerivationExtension {
		t.Fatalf("Ext mask = %v, want %v", got, model.DerivationExtension)
	}

	restOff := ancestors.Offsets[restID]
	restLen := ancestors.Lengths[restID]
	if restLen != 2 {
		t.Fatalf("Restrict ancestors length = %d, want 2", restLen)
	}
	if got := ancestors.IDs[restOff]; got != extID {
		t.Fatalf("Restrict ancestor[0] ID = %d, want %d", got, extID)
	}
	if got := ancestors.Masks[restOff]; got != model.DerivationRestriction {
		t.Fatalf("Restrict mask[0] = %v, want %v", got, model.DerivationRestriction)
	}
	if got := ancestors.IDs[restOff+1]; got != baseID {
		t.Fatalf("Restrict ancestor[1] ID = %d, want %d", got, baseID)
	}
	wantMask := model.DerivationRestriction | model.DerivationExtension
	if got := ancestors.Masks[restOff+1]; got != wantMask {
		t.Fatalf("Restrict mask[1] = %v, want %v", got, wantMask)
	}
}

func TestBuildAncestorsSimpleTypeInlineRestrictionBase(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:anc"
           xmlns:tns="urn:anc">
  <xs:simpleType name="Outer">
    <xs:restriction>
      <xs:simpleType>
        <xs:restriction base="xs:int"/>
      </xs:simpleType>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	outerQName := model.QName{Namespace: "urn:anc", Local: "Outer"}
	outerType, ok := sch.TypeDefs[outerQName].(*model.SimpleType)
	if !ok {
		t.Fatalf("Outer type = %T, want *model.SimpleType", sch.TypeDefs[outerQName])
	}
	if outerType.Restriction == nil || outerType.Restriction.SimpleType == nil {
		t.Fatalf("Outer restriction simpleType not resolved")
	}

	registry, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}
	ancestors, err := schema.BuildAncestors(sch, registry)
	if err != nil {
		t.Fatalf("BuildAncestors error = %v", err)
	}

	outerID := registry.Types[outerQName]
	inlineID, ok := registry.LookupAnonymousTypeID(outerType.Restriction.SimpleType)
	if !ok {
		t.Fatalf("inline restriction type ID not found")
	}
	outerOff := ancestors.Offsets[outerID]
	if got := ancestors.Lengths[outerID]; got != 1 {
		t.Fatalf("Outer ancestors length = %d, want 1", got)
	}
	if got := ancestors.IDs[outerOff]; got != inlineID {
		t.Fatalf("Outer ancestor ID = %d, want %d", got, inlineID)
	}
	if got := ancestors.Masks[outerOff]; got != model.DerivationRestriction {
		t.Fatalf("Outer mask = %v, want %v", got, model.DerivationRestriction)
	}
}

func TestBuildAncestorsSimpleTypeResolvedBaseFallback(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:anc"
           xmlns:tns="urn:anc">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="tns:Base"/>
  </xs:simpleType>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	baseQName := model.QName{Namespace: "urn:anc", Local: "Base"}
	derivedQName := model.QName{Namespace: "urn:anc", Local: "Derived"}
	baseType := sch.TypeDefs[baseQName]
	derivedType, ok := sch.TypeDefs[derivedQName].(*model.SimpleType)
	if !ok {
		t.Fatalf("Derived type = %T, want *model.SimpleType", sch.TypeDefs[derivedQName])
	}

	// exercise the ResolvedBase fallback path used by runtime assembly.
	derivedType.Restriction.Base = model.QName{}
	derivedType.ResolvedBase = baseType

	registry, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}
	ancestors, err := schema.BuildAncestors(sch, registry)
	if err != nil {
		t.Fatalf("BuildAncestors error = %v", err)
	}

	baseID := registry.Types[baseQName]
	derivedID := registry.Types[derivedQName]
	derivedOff := ancestors.Offsets[derivedID]
	if got := ancestors.Lengths[derivedID]; got != 1 {
		t.Fatalf("Derived ancestors length = %d, want 1", got)
	}
	if got := ancestors.IDs[derivedOff]; got != baseID {
		t.Fatalf("Derived ancestor ID = %d, want %d", got, baseID)
	}
	if got := ancestors.Masks[derivedOff]; got != model.DerivationRestriction {
		t.Fatalf("Derived mask = %v, want %v", got, model.DerivationRestriction)
	}
}
