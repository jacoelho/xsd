package types_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestNewAtomicSimpleType_MissingRestriction(t *testing.T) {
	if _, err := types.NewAtomicSimpleType(types.QName{Local: "NoDerivation"}, "", nil); err == nil {
		t.Fatal("expected error for missing restriction")
	}
}

func TestNewAtomicSimpleType_RestrictionMissingBase(t *testing.T) {
	if _, err := types.NewAtomicSimpleType(types.QName{Local: "MissingBase"}, "", &types.Restriction{}); err == nil {
		t.Fatal("expected error for restriction without base type")
	}
}

func TestNewListSimpleType_ListMissingItemType(t *testing.T) {
	if _, err := types.NewListSimpleType(types.QName{Local: "MissingItem"}, "", &types.ListType{}, nil); err == nil {
		t.Fatal("expected error for list without item type")
	}
}

func TestNewUnionSimpleType_UnionMissingMembers(t *testing.T) {
	if _, err := types.NewUnionSimpleType(types.QName{Local: "MissingMembers"}, "", &types.UnionType{}); err == nil {
		t.Fatal("expected error for union without member types")
	}
}

func TestNewAtomicSimpleType_FacetNotApplicable(t *testing.T) {
	st, err := types.NewAtomicSimpleType(
		types.QName{Local: "BadFacet"},
		"",
		&types.Restriction{
			Base: types.QName{
				Namespace: types.XSDNamespace,
				Local:     string(types.TypeNameString),
			},
			Facets: []any{
				&types.FractionDigits{Value: 2},
			},
		},
	)
	if err == nil {
		t.Fatal("expected error for incompatible facet")
	}
	if st != nil {
		t.Fatalf("expected constructor to fail, got %#v", st)
	}
}

func TestNewAtomicSimpleType_DefersFacetApplicabilityForUnresolvedInlineBase(t *testing.T) {
	inlineBase, err := types.NewAtomicSimpleType(
		types.QName{Namespace: "http://example.com", Local: "InlineBase"},
		"http://example.com",
		&types.Restriction{
			Base: types.QName{Namespace: "http://example.com", Local: "LaterType"},
		},
	)
	if err != nil {
		t.Fatalf("inline base: %v", err)
	}

	_, err = types.NewAtomicSimpleType(
		types.QName{Namespace: "http://example.com", Local: "OuterType"},
		"http://example.com",
		&types.Restriction{
			SimpleType: inlineBase,
			Facets: []any{
				&types.FractionDigits{Value: 2},
			},
		},
	)
	if err != nil {
		t.Fatalf("expected no error for unresolved inline base applicability, got %v", err)
	}
}
