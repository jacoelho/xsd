package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestRawDecimalAdmittedBoundFallbacks(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, min, max string
		fraction       bool
		cases          []struct{ input, message string }
	}{
		{name: "negative", min: "-2", max: "-1", cases: []struct{ input, message string }{
			{"-1.5", ""}, {"0", "upper bound facet failed"},
		}},
		{name: "digits", min: "0", max: "10", fraction: true, cases: []struct{ input, message string }{
			{"1.20", ""}, {"1.25", "digit facet failed"},
		}},
		{name: "fractional", min: "+000.5000", max: "+000.5000", cases: []struct{ input, message string }{
			{".5", ""}, {".4999", "minInclusive facet failed"}, {".5001", "maxInclusive facet failed"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			facets := value.FacetSpec{
				MinInclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: test.min, Type: value.NoType}, Present: true},
				MaxInclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: test.max, Type: value.NoType}, Present: true},
			}
			if test.fraction {
				facets.FractionDigits = value.CardinalityFacet{Value: 1, Present: true}
			}
			if completeErr := builder.Complete(id, value.TypeSpec{
				Variety: value.Atomic, Primitive: value.PrimitiveDecimal,
				Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
				Base: value.NoType, ListItem: value.NoType, Facets: facets,
			}); completeErr != nil {
				t.Fatal(completeErr)
			}
			union, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if completeErr := builder.Complete(union, value.TypeSpec{
				Variety: value.Union, Union: []value.TypeID{id, value.BuiltinType(value.PrimitiveString)},
				Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
				Base: value.NoType, ListItem: value.NoType,
			}); completeErr != nil {
				t.Fatal(completeErr)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			var scratch value.Scratch
			for _, tc := range test.cases {
				got, err := program.ValidateBytes(id, []byte(tc.input), value.Resolver{}, 0, 16<<20, &scratch)
				if tc.message == "" {
					if err != nil || got.Type() != id || got.SelectedType() != value.NoType || got.CanonicalText() != "" || got.IdentityKey() != "" {
						t.Fatalf("ValidateBytes(%q) = %#v, %v", tc.input, got, err)
					}
				} else {
					if err == nil || err.Error() != tc.message {
						t.Fatalf("ValidateBytes(%q) = %v, want %q", tc.input, err, tc.message)
					}
					if test.name != "fractional" && !errors.Is(err, value.ErrFacet) {
						t.Fatalf("ValidateBytes(%q) error %v lacks facet classification", tc.input, err)
					}
				}
				selected, unionErr := program.ValidateBytes(union, []byte(tc.input), value.Resolver{}, 0, 16<<20, &scratch)
				want := id
				if tc.message != "" {
					want = value.BuiltinType(value.PrimitiveString)
				}
				if unionErr != nil || selected.Type() != union || selected.SelectedType() != want {
					t.Fatalf("union ValidateBytes(%q) type/selection=%d/%d, error=%v; want %d/%d", tc.input, selected.Type(), selected.SelectedType(), unionErr, union, want)
				}
			}
		})
	}
}
