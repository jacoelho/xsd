package model_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/facetvalue"
	model "github.com/jacoelho/xsd/internal/model"
)

func TestSimpleTypeValidateAppliesFacets(t *testing.T) {
	base := builtins.Get(model.TypeNameInteger)
	if base == nil {
		t.Fatalf("expected builtin integer")
	}
	maxFacet, err := facetvalue.NewMaxInclusive("10", base)
	if err != nil {
		t.Fatalf("newMaxInclusive: %v", err)
	}
	st, err := model.NewAtomicSimpleType(model.QName{Namespace: "urn:test", Local: "Int10"}, "urn:test", &model.Restriction{
		Base:   model.QName{Namespace: model.XSDNamespace, Local: "integer"},
		Facets: []any{maxFacet},
	})
	if err != nil {
		t.Fatalf("NewAtomicSimpleType: %v", err)
	}
	if err := st.Validate("11"); err == nil {
		t.Fatalf("expected facet validation error")
	}
	if err := st.Validate("10"); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestSimpleTypeParseValueAppliesFacets(t *testing.T) {
	base := builtins.Get(model.TypeNameInteger)
	if base == nil {
		t.Fatalf("expected builtin integer")
	}
	maxFacet, err := facetvalue.NewMaxInclusive("10", base)
	if err != nil {
		t.Fatalf("newMaxInclusive: %v", err)
	}
	st, err := model.NewAtomicSimpleType(model.QName{Namespace: "urn:test", Local: "Int10"}, "urn:test", &model.Restriction{
		Base:   model.QName{Namespace: model.XSDNamespace, Local: "integer"},
		Facets: []any{maxFacet},
	})
	if err != nil {
		t.Fatalf("NewAtomicSimpleType: %v", err)
	}
	if _, err := st.ParseValue("11"); err == nil {
		t.Fatalf("expected ParseValue facet error")
	}
	if _, err := st.ParseValue("10"); err != nil {
		t.Fatalf("unexpected ParseValue error: %v", err)
	}
}

func TestSimpleTypeValidateUnionMembers(t *testing.T) {
	base := builtins.Get(model.TypeNameInteger)
	if base == nil {
		t.Fatalf("expected builtin integer")
	}
	maxFacet, err := facetvalue.NewMaxInclusive("10", base)
	if err != nil {
		t.Fatalf("newMaxInclusive: %v", err)
	}
	int10, err := model.NewAtomicSimpleType(model.QName{Namespace: "urn:test", Local: "Int10"}, "urn:test", &model.Restriction{
		Base:   model.QName{Namespace: model.XSDNamespace, Local: "integer"},
		Facets: []any{maxFacet},
	})
	if err != nil {
		t.Fatalf("NewAtomicSimpleType: %v", err)
	}
	union, err := model.NewUnionSimpleType(model.QName{Namespace: "urn:test", Local: "Union"}, "urn:test", &model.UnionType{
		MemberTypes: []model.QName{int10.QName},
	})
	if err != nil {
		t.Fatalf("NewUnionSimpleType: %v", err)
	}
	union.MemberTypes = []model.Type{int10}

	if err := union.Validate("11"); err == nil {
		t.Fatalf("expected union member validation error")
	}
	if err := union.Validate("10"); err != nil {
		t.Fatalf("unexpected union validation error: %v", err)
	}
}

func TestSimpleTypeValidateListMembers(t *testing.T) {
	item, err := builtins.NewSimpleType(model.TypeNameInteger)
	if err != nil {
		t.Fatalf("NewBuiltinSimpleType: %v", err)
	}
	list, err := model.NewListSimpleType(model.QName{Namespace: "urn:test", Local: "IntList"}, "urn:test", &model.ListType{
		InlineItemType: item,
	}, nil)
	if err != nil {
		t.Fatalf("NewListSimpleType: %v", err)
	}
	if err := list.Validate("1 2 3"); err != nil {
		t.Fatalf("unexpected list validation error: %v", err)
	}
	if err := list.Validate("1 a"); err == nil {
		t.Fatalf("expected list item validation error")
	}
	if err := list.Validate(""); err != nil {
		t.Fatalf("unexpected empty list validation error: %v", err)
	}
}

func TestSimpleTypeValidateWithContextQNameEnumeration(t *testing.T) {
	enum := model.NewEnumeration([]string{"p:red", "p:blue"})
	enum.SetValueContexts([]map[string]string{
		{"p": "urn:colors"},
		{"p": "urn:colors"},
	})
	st, err := model.NewAtomicSimpleType(model.QName{Namespace: "urn:test", Local: "ColorQN"}, "urn:test", &model.Restriction{
		Base:   model.QName{Namespace: model.XSDNamespace, Local: "QName"},
		Facets: []any{enum},
	})
	if err != nil {
		t.Fatalf("NewAtomicSimpleType: %v", err)
	}

	if err := st.ValidateWithContext("c:red", map[string]string{"c": "urn:colors"}); err != nil {
		t.Fatalf("expected QName enum to match across prefixes: %v", err)
	}
	if err := st.ValidateWithContext("c:green", map[string]string{"c": "urn:colors"}); err == nil {
		t.Fatalf("expected QName enum mismatch for non-enumerated value")
	}
	if err := st.ValidateWithContext("c:red", map[string]string{"c": "urn:other"}); err == nil {
		t.Fatalf("expected QName enum mismatch for different namespace")
	}
}
