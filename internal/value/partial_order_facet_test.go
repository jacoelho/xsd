package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestOrderedFacetsRejectIncomparableValues(t *testing.T) {
	for _, test := range []struct {
		name           string
		primitive      value.PrimitiveKind
		bound, lexical string
	}{
		{"duration", value.PrimitiveDuration, "P365D", "P1Y"},
		{"dateTime", value.PrimitiveDateTime, "2000-01-01T12:00:00Z", "2000-01-01T12:00:00"},
	} {
		for _, facet := range []string{"minInclusive", "minExclusive", "maxInclusive", "maxExclusive"} {
			t.Run(test.name+"/"+facet, func(t *testing.T) {
				literal := value.BoundFacet{Present: true, Lexical: test.bound, Type: value.NoType}
				var facets value.FacetSpec
				switch facet {
				case "minInclusive":
					facets.MinInclusive = literal
				case "minExclusive":
					facets.MinExclusive = literal
				case "maxInclusive":
					facets.MaxInclusive = literal
				case "maxExclusive":
					facets.MaxExclusive = literal
				}
				builder := value.NewBuilder(value.BuilderOptions{})
				id, err := builder.Add(atomicFacetType(test.primitive, facets))
				if err != nil {
					t.Fatal(err)
				}
				program, err := builder.Seal()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := program.Validate(id, test.lexical, value.Resolver{}, 0, 16<<20, nil); !errors.Is(err, value.ErrFacet) {
					t.Fatalf("incomparable value accepted by %s: %v", facet, err)
				}
				if _, err := program.ValidateBytes(id, []byte(test.lexical), value.Resolver{}, 0, 16<<20, nil); !errors.Is(err, value.ErrFacet) {
					t.Fatalf("incomparable raw value accepted by %s: %v", facet, err)
				}
			})
		}
	}
}
