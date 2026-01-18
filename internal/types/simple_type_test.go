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

func TestListSimpleTypeMeasureLength_XMLWhitespace(t *testing.T) {
	listType, err := types.NewListSimpleType(
		types.QName{Namespace: "http://example.com", Local: "ListType"},
		"http://example.com",
		&types.ListType{
			ItemType: types.QName{
				Namespace: types.XSDNamespace,
				Local:     string(types.TypeNameNMTOKEN),
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("list type: %v", err)
	}

	restricted := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "RestrictedListType",
		},
		Restriction: &types.Restriction{
			Base: listType.QName,
		},
		ResolvedBase: listType,
	}

	tests := []struct {
		name  string
		value string
		want  int
	}{
		{name: "empty", value: "", want: 0},
		{name: "whitespace only", value: " \t\r\n", want: 0},
		{name: "single item", value: "a", want: 1},
		{name: "space separated", value: "a b c", want: 3},
		{name: "tab separated", value: "a\tb", want: 2},
		{name: "lf separated", value: "a\nb", want: 2},
		{name: "cr separated", value: "a\rb", want: 2},
		{name: "crlf separated", value: "a\r\nb", want: 2},
		{name: "mixed separators", value: " \ta\r\nb\nc\t ", want: 3},
		{name: "non-xml nbsp", value: "a\u00A0b", want: 1},
		{name: "non-xml nel", value: "a\u0085b", want: 1},
		{name: "non-xml ls", value: "a\u2028b", want: 1},
		{name: "non-xml ps", value: "a\u2029b", want: 1},
		{name: "non-xml thin space", value: "a\u2009b", want: 1},
		{name: "non-xml vt", value: "a\u000bb", want: 1},
		{name: "non-xml ff", value: "a\u000cb", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listType.MeasureLength(tt.value); got != tt.want {
				t.Fatalf("MeasureLength(list %q) = %d, want %d", tt.value, got, tt.want)
			}
			if got := restricted.MeasureLength(tt.value); got != tt.want {
				t.Fatalf("MeasureLength(restricted %q) = %d, want %d", tt.value, got, tt.want)
			}
		})
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
