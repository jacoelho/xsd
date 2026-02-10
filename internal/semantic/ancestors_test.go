package semantic_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	schema "github.com/jacoelho/xsd/internal/semantic"
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
