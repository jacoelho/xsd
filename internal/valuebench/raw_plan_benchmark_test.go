package valuebench

import (
	"errors"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func BenchmarkPublishedRawDecimalBounds(b *testing.B) {
	for _, test := range []struct {
		name, min, max, input, message string
		fraction                       bool
	}{
		{name: "NoBounds", input: "42.50"},
		{name: "MinOnly", min: ".5", input: ".5"},
		{name: "MaxOnly", max: "+000.5000", input: ".5"},
		{name: "Both", min: "0", max: "999999", input: "42.50"},
		{name: "NegativeFallback", min: "-2", max: "-1", input: "-1.5"},
		{name: "WhitespaceFallback", min: "0", max: "999999", input: " \t42.50\n"},
		{name: "MaxReject", min: "0", max: "999999", input: "1000000", message: "maxInclusive facet failed"},
		{name: "UnsupportedFractionDigits", min: "0", max: "10", fraction: true, input: "1.25", message: "digit facet failed"},
	} {
		b.Run(test.name, func(b *testing.B) {
			builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				b.Fatal(err)
			}
			var facets valuepkg.FacetSpec
			if test.min != "" {
				facets.MinInclusive = valuepkg.BoundFacet{Lexical: test.min, Type: valuepkg.NoType, Present: true}
			}
			if test.max != "" {
				facets.MaxInclusive = valuepkg.BoundFacet{Lexical: test.max, Type: valuepkg.NoType, Present: true}
			}
			if test.fraction {
				facets.FractionDigits = valuepkg.CardinalityFacet{Value: 1, Present: true}
			}
			if completeErr := builder.Complete(id, valuepkg.TypeSpec{
				Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveDecimal,
				Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
				Base: valuepkg.NoType, ListItem: valuepkg.NoType, Facets: facets,
			}); completeErr != nil {
				b.Fatal(completeErr)
			}
			program, err := builder.Seal()
			if err != nil {
				b.Fatal(err)
			}
			raw := []byte(test.input)
			var scratch valuepkg.Scratch
			check := func(input, message string, facet bool) valuepkg.Value {
				got, validationErr := program.ValidateBytes(id, []byte(input), valuepkg.Resolver{}, 0, 16<<20, &scratch)
				if message != "" {
					if validationErr == nil || validationErr.Error() != message || facet && !errors.Is(validationErr, valuepkg.ErrFacet) {
						b.Fatalf("ValidateBytes(%q) = %v, want %q", input, validationErr, message)
					}
					return got
				}
				if validationErr != nil {
					b.Fatal(validationErr)
				}
				checkRawPlanResult(b, got, id)
				return got
			}
			preflight := func() {
				check(test.input, test.message, test.fraction)
				if test.name == "NegativeFallback" {
					check("0", "upper bound facet failed", true)
					check("-1.5", "", false)
				}
				if test.fraction {
					check("1.25", "digit facet failed", true)
					check("1.20", "", false)
				}
			}
			preflight()
			var last valuepkg.Value
			b.ReportAllocs()
			for b.Loop() {
				last, err = program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, &scratch)
				if test.message == "" {
					if err != nil {
						b.Fatal(err)
					}
				} else if err == nil || err.Error() != test.message || test.fraction && !errors.Is(err, valuepkg.ErrFacet) {
					b.Fatalf("ValidateBytes(%q) = %v, want %q", test.input, err, test.message)
				}
			}
			if test.message == "" {
				checkRawPlanResult(b, last, id)
			}
			preflight()
		})
	}
}

func checkRawPlanResult(tb testing.TB, got valuepkg.Value, id valuepkg.TypeID) {
	tb.Helper()
	if got.Type() != id || got.SelectedType() != valuepkg.NoType || got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
		tb.Fatalf("unprojected decimal result = %#v", got)
	}
}
