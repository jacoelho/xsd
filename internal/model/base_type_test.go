package model

import (
	"testing"
)

func TestBaseType_ForRestriction(t *testing.T) {
	// test that base type is resolved for restriction
	baseType := mustBuiltinSimpleType(t, TypeNameDecimal)

	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "MyDecimal",
		},
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
	// list types don't have a base type in the same way, but itemType should be resolved
	itemType := mustBuiltinSimpleType(t, TypeNameString)

	listType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "StringList",
		},
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
	// union types have member types, not a single base type
	member1 := mustBuiltinSimpleType(t, TypeNameString)
	member2 := mustBuiltinSimpleType(t, TypeNameInteger)

	unionType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "StringOrInteger",
		},
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
