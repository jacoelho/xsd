package types_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestNewSimpleTypeFromParsed_MissingDerivation(t *testing.T) {
	st := &types.SimpleType{
		QName: types.QName{Local: "NoDerivation"},
	}
	st.SetVariety(types.AtomicVariety)

	if _, err := types.NewSimpleTypeFromParsed(st); err == nil {
		t.Fatal("expected error for missing derivation")
	}
}

func TestNewSimpleTypeFromParsed_RestrictionMissingBase(t *testing.T) {
	st := &types.SimpleType{
		QName:       types.QName{Local: "MissingBase"},
		Restriction: &types.Restriction{},
	}
	st.SetVariety(types.AtomicVariety)

	if _, err := types.NewSimpleTypeFromParsed(st); err == nil {
		t.Fatal("expected error for restriction without base type")
	}
}

func TestNewSimpleTypeFromParsed_ListMissingItemType(t *testing.T) {
	st := &types.SimpleType{
		QName: types.QName{Local: "MissingItem"},
		List:  &types.ListType{},
	}
	st.SetVariety(types.ListVariety)

	if _, err := types.NewSimpleTypeFromParsed(st); err == nil {
		t.Fatal("expected error for list without item type")
	}
}

func TestNewSimpleTypeFromParsed_UnionMissingMembers(t *testing.T) {
	st := &types.SimpleType{
		QName: types.QName{Local: "MissingMembers"},
		Union: &types.UnionType{},
	}
	st.SetVariety(types.UnionVariety)

	if _, err := types.NewSimpleTypeFromParsed(st); err == nil {
		t.Fatal("expected error for union without member types")
	}
}

func TestNewSimpleTypeFromParsed_FacetNotApplicable(t *testing.T) {
	st := &types.SimpleType{
		QName: types.QName{Local: "BadFacet"},
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: types.XSDNamespace,
				Local:     string(types.TypeNameString),
			},
			Facets: []any{
				&types.FractionDigits{Value: 2},
			},
		},
	}
	st.SetVariety(types.AtomicVariety)

	if _, err := types.NewSimpleTypeFromParsed(st); err == nil {
		t.Fatal("expected error for incompatible facet")
	}
}
