package semanticresolve

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestResolveSimpleTypeRestriction(t *testing.T) {
	schema := parser.NewSchema()
	baseQName := model.QName{Local: "base"}
	base := &model.SimpleType{
		QName: baseQName,
		Restriction: &model.Restriction{
			Base: model.QName{Namespace: model.XSDNamespace, Local: "string"},
		},
	}
	derivedQName := model.QName{Local: "derived"}
	derived := &model.SimpleType{
		QName: derivedQName,
		Restriction: &model.Restriction{
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
	itemQName := model.QName{Local: "item"}
	item := &model.SimpleType{
		QName: itemQName,
		Restriction: &model.Restriction{
			Base: model.QName{Namespace: model.XSDNamespace, Local: "string"},
		},
	}
	listQName := model.QName{Local: "list"}
	list := &model.SimpleType{
		QName: listQName,
		List: &model.ListType{
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
	if list.WhiteSpace() != model.WhiteSpaceCollapse {
		t.Fatalf("expected list whitespace collapse, got %v", list.WhiteSpace())
	}
}

func TestResolveSimpleTypeUnion(t *testing.T) {
	schema := parser.NewSchema()
	leftQName := model.QName{Local: "left"}
	left := &model.SimpleType{
		QName: leftQName,
		Restriction: &model.Restriction{
			Base: model.QName{Namespace: model.XSDNamespace, Local: "string"},
		},
	}
	rightQName := model.QName{Local: "right"}
	right := &model.SimpleType{
		QName: rightQName,
		Restriction: &model.Restriction{
			Base: model.QName{Namespace: model.XSDNamespace, Local: "int"},
		},
	}
	unionQName := model.QName{Local: "union"}
	union := &model.SimpleType{
		QName: unionQName,
		Union: &model.UnionType{
			MemberTypes: []model.QName{leftQName, rightQName},
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
