package types

import (
	"testing"
)

func TestBaseType_ForRestriction(t *testing.T) {
	// Test that base type is resolved for restriction
	baseType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameDecimal),
		},
		// Variety set via SetVariety
	}
	baseType.MarkBuiltin()
	baseType.SetVariety(AtomicVariety)
	baseType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameDecimal))

	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "MyDecimal",
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: baseType.QName,
		},
	}
	derivedType.ResolvedBase = baseType

	// BaseType() should never be nil now (returns anySimpleType if ResolvedBase is nil)
	if derivedType.BaseType() != baseType {
		t.Fatal("BaseType should be set to baseType")
	}
	if derivedType.BaseType().Name().Local != string(TypeNameDecimal) {
		t.Errorf("BaseType = %q, want %q", derivedType.BaseType().Name().Local, string(TypeNameDecimal))
	}
}

func TestBaseType_ForListType(t *testing.T) {
	// List types don't have a base type in the same way, but itemType should be resolved
	itemType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameString),
		},
		// Variety set via SetVariety
	}
	itemType.MarkBuiltin()
	itemType.SetVariety(AtomicVariety)
	itemType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameString))

	listType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "StringList",
		},
		// Variety set via SetVariety
		List: &ListType{
			ItemType: itemType.QName,
		},
		ItemType: itemType,
	}

	if listType.ItemType == nil {
		t.Fatal("ItemType should be set")
	}
	if listType.ItemType.Name().Local != string(TypeNameString) {
		t.Errorf("ItemType = %q, want %q", listType.ItemType.Name().Local, string(TypeNameString))
	}
}

func TestBaseType_ForUnionType(t *testing.T) {
	// Union types have member types, not a single base type
	member1 := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameString),
		},
		// Variety set via SetVariety
	}
	member1.MarkBuiltin()
	member1.SetVariety(AtomicVariety)
	member1.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameString))

	member2 := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "integer",
		},
		// Variety set via SetVariety
	}
	member2.MarkBuiltin()
	member2.SetVariety(AtomicVariety)

	unionType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "StringOrInteger",
		},
		// Variety set via SetVariety
		Union: &UnionType{
			MemberTypes: []QName{
				member1.QName,
				member2.QName,
			},
		},
		MemberTypes: []Type{member1, member2},
	}

	if len(unionType.MemberTypes) != 2 {
		t.Fatalf("MemberTypes length = %d, want 2", len(unionType.MemberTypes))
	}
	if unionType.MemberTypes[0].Name().Local != "string" {
		t.Errorf("MemberTypes[0] = %q, want %q", unionType.MemberTypes[0].Name().Local, "string")
	}
}
