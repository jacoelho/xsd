package model_test

import (
	"errors"
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

func TestSimpleTypeValidateUnionInlineMembers(t *testing.T) {
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
	union, err := model.NewUnionSimpleType(model.QName{Namespace: "urn:test", Local: "InlineUnion"}, "urn:test", &model.UnionType{
		InlineTypes: []*model.SimpleType{int10},
	})
	if err != nil {
		t.Fatalf("NewUnionSimpleType: %v", err)
	}

	if err := union.Validate("11"); err == nil {
		t.Fatalf("expected union inline member validation error")
	}
	if err := union.Validate("10"); err != nil {
		t.Fatalf("unexpected union inline validation error: %v", err)
	}
}

func TestSimpleTypeValidateUnionMemberQNames(t *testing.T) {
	union, err := model.NewUnionSimpleType(model.QName{Namespace: "urn:test", Local: "QNameUnion"}, "urn:test", &model.UnionType{
		MemberTypes: []model.QName{
			{Namespace: model.XSDNamespace, Local: "int"},
		},
	})
	if err != nil {
		t.Fatalf("NewUnionSimpleType: %v", err)
	}

	if err := union.Validate("12"); err != nil {
		t.Fatalf("unexpected union QName member validation error: %v", err)
	}
	if err := union.Validate("abc"); err == nil {
		t.Fatalf("expected union QName member validation error")
	}
}

func TestValidateSimpleTypeWithOptionsUnionCycleThenMatchSucceeds(t *testing.T) {
	stringType := builtins.Get(model.TypeNameString)
	if stringType == nil {
		t.Fatalf("expected builtin string")
	}
	cycleMember := newSelfReferentialListSimpleType(model.QName{Namespace: "urn:test", Local: "CycleMember"})
	union := &model.SimpleType{
		QName:       model.QName{Namespace: "urn:test", Local: "CycleThenString"},
		Union:       &model.UnionType{},
		MemberTypes: []model.Type{cycleMember, stringType},
	}

	cycleErr := errors.New("cycle")
	var calledUnionNoMatch bool
	err := model.ValidateSimpleTypeWithOptions(union, "abc", nil, model.SimpleTypeValidationOptions{
		CycleError: cycleErr,
		UnionNoMatch: func(_ *model.SimpleType, _ string, _ error, _ bool) error {
			calledUnionNoMatch = true
			return errors.New("unexpected union no match")
		},
	})
	if err != nil {
		t.Fatalf("ValidateSimpleTypeWithOptions() error = %v, want nil", err)
	}
	if calledUnionNoMatch {
		t.Fatalf("UnionNoMatch callback called after union member validation succeeded")
	}
}

func TestValidateSimpleTypeWithOptionsUnionAllCycleReportsNoMatch(t *testing.T) {
	cycleMember := newSelfReferentialListSimpleType(model.QName{Namespace: "urn:test", Local: "CycleOnly"})
	union := &model.SimpleType{
		QName:       model.QName{Namespace: "urn:test", Local: "CycleUnion"},
		Union:       &model.UnionType{},
		MemberTypes: []model.Type{cycleMember},
	}

	cycleErr := errors.New("cycle")
	unionNoMatchErr := errors.New("union no match")
	var (
		gotSawCycle bool
		gotFirstErr error
	)
	err := model.ValidateSimpleTypeWithOptions(union, "abc", nil, model.SimpleTypeValidationOptions{
		CycleError: cycleErr,
		UnionNoMatch: func(_ *model.SimpleType, _ string, firstErr error, sawCycle bool) error {
			gotFirstErr = firstErr
			gotSawCycle = sawCycle
			return unionNoMatchErr
		},
	})
	if !errors.Is(err, unionNoMatchErr) {
		t.Fatalf("ValidateSimpleTypeWithOptions() error = %v, want %v", err, unionNoMatchErr)
	}
	if !gotSawCycle {
		t.Fatalf("UnionNoMatch sawCycle = false, want true")
	}
	if gotFirstErr != nil {
		t.Fatalf("UnionNoMatch firstErr = %v, want nil", gotFirstErr)
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

func TestListItemTypeWithResolverResolvesNamedRestrictionBase(t *testing.T) {
	baseList := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "BaseList"},
		List: &model.ListType{
			ItemType: model.QName{Namespace: model.XSDNamespace, Local: "int"},
		},
	}
	derived := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "DerivedList"},
		Restriction: &model.Restriction{
			Base: baseList.QName,
		},
	}

	item, ok := model.ListItemTypeWithResolver(derived, func(name model.QName) model.Type {
		if name == baseList.QName {
			return baseList
		}
		return nil
	})
	if !ok || item == nil {
		t.Fatalf("expected list item type from named restriction base")
	}
	if item.Name().Local != "int" {
		t.Fatalf("item type local name = %q, want %q", item.Name().Local, "int")
	}
}

func TestListItemTypeHandlesTypedNilBase(t *testing.T) {
	var nilBase *model.BuiltinType
	list := &model.SimpleType{
		QName:        model.QName{Namespace: "urn:test", Local: "TypedNilBaseList"},
		List:         &model.ListType{},
		ResolvedBase: nilBase,
	}

	if item, ok := model.ListItemType(list); ok || item != nil {
		t.Fatalf("ListItemType() = (%v, %v), want (nil, false)", item, ok)
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

func newSelfReferentialListSimpleType(name model.QName) *model.SimpleType {
	st := &model.SimpleType{
		QName: name,
		List:  &model.ListType{},
	}
	st.ItemType = st
	return st
}
