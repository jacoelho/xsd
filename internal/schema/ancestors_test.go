package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestBuildAncestorsMasks(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:anc"
           xmlns:tns="urn:anc">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Ext">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="b" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="Restrict">
    <xs:complexContent>
      <xs:restriction base="tns:Ext">
        <xs:sequence>
          <xs:element name="a" type="xs:string"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	schema := mustParseSchema(t, schemaXML)
	registry, err := AssignIDs(schema)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}

	ancestors, err := BuildAncestors(schema, registry)
	if err != nil {
		t.Fatalf("BuildAncestors error = %v", err)
	}

	baseQName := types.QName{Namespace: "urn:anc", Local: "Base"}
	extQName := types.QName{Namespace: "urn:anc", Local: "Ext"}
	restQName := types.QName{Namespace: "urn:anc", Local: "Restrict"}

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
	if got := ancestors.Masks[extOff]; got != types.DerivationExtension {
		t.Fatalf("Ext mask = %v, want %v", got, types.DerivationExtension)
	}

	restOff := ancestors.Offsets[restID]
	restLen := ancestors.Lengths[restID]
	if restLen != 2 {
		t.Fatalf("Restrict ancestors length = %d, want 2", restLen)
	}
	if got := ancestors.IDs[restOff]; got != extID {
		t.Fatalf("Restrict ancestor[0] ID = %d, want %d", got, extID)
	}
	if got := ancestors.Masks[restOff]; got != types.DerivationRestriction {
		t.Fatalf("Restrict mask[0] = %v, want %v", got, types.DerivationRestriction)
	}
	if got := ancestors.IDs[restOff+1]; got != baseID {
		t.Fatalf("Restrict ancestor[1] ID = %d, want %d", got, baseID)
	}
	wantMask := types.DerivationRestriction | types.DerivationExtension
	if got := ancestors.Masks[restOff+1]; got != wantMask {
		t.Fatalf("Restrict mask[1] = %v, want %v", got, wantMask)
	}
}
