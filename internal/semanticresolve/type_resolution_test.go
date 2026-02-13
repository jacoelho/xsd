package semanticresolve

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestResolveSimpleTypeRestriction(t *testing.T) {
	schema := parser.NewSchema()
	baseQName := types.QName{Local: "base"}
	base := &types.SimpleType{
		QName: baseQName,
		Restriction: &types.Restriction{
			Base: types.QName{Namespace: types.XSDNamespace, Local: "string"},
		},
	}
	derivedQName := types.QName{Local: "derived"}
	derived := &types.SimpleType{
		QName: derivedQName,
		Restriction: &types.Restriction{
			Base: baseQName,
		},
	}
	schema.TypeDefs[baseQName] = base
	schema.TypeDefs[derivedQName] = derived

	res := NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve restriction: %v", err)
	}
	if derived.ResolvedBase != base {
		t.Fatalf("expected derived base to resolve to %v", baseQName)
	}
	if base.ResolvedBase == nil {
		t.Fatalf("expected base type to resolve its built-in base")
	}
}

func TestResolveSimpleTypeList(t *testing.T) {
	schema := parser.NewSchema()
	itemQName := types.QName{Local: "item"}
	item := &types.SimpleType{
		QName: itemQName,
		Restriction: &types.Restriction{
			Base: types.QName{Namespace: types.XSDNamespace, Local: "string"},
		},
	}
	listQName := types.QName{Local: "list"}
	list := &types.SimpleType{
		QName: listQName,
		List: &types.ListType{
			ItemType: itemQName,
		},
	}
	schema.TypeDefs[itemQName] = item
	schema.TypeDefs[listQName] = list

	res := NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve list: %v", err)
	}
	if list.ItemType != item {
		t.Fatalf("expected list item type to resolve to %v", itemQName)
	}
	if list.WhiteSpace() != types.WhiteSpaceCollapse {
		t.Fatalf("expected list whitespace collapse, got %v", list.WhiteSpace())
	}
}

func TestResolveSimpleTypeUnion(t *testing.T) {
	schema := parser.NewSchema()
	leftQName := types.QName{Local: "left"}
	left := &types.SimpleType{
		QName: leftQName,
		Restriction: &types.Restriction{
			Base: types.QName{Namespace: types.XSDNamespace, Local: "string"},
		},
	}
	rightQName := types.QName{Local: "right"}
	right := &types.SimpleType{
		QName: rightQName,
		Restriction: &types.Restriction{
			Base: types.QName{Namespace: types.XSDNamespace, Local: "int"},
		},
	}
	unionQName := types.QName{Local: "union"}
	union := &types.SimpleType{
		QName: unionQName,
		Union: &types.UnionType{
			MemberTypes: []types.QName{leftQName, rightQName},
		},
	}
	schema.TypeDefs[leftQName] = left
	schema.TypeDefs[rightQName] = right
	schema.TypeDefs[unionQName] = union

	res := NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve union: %v", err)
	}
	if len(union.MemberTypes) != 2 {
		t.Fatalf("expected 2 union member types, got %d", len(union.MemberTypes))
	}
	if union.MemberTypes[0] != left || union.MemberTypes[1] != right {
		t.Fatalf("expected union member types to resolve to left/right")
	}
}
