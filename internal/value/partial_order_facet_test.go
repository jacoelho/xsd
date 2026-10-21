package value_test

import (
	"errors"
	"strings"
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
				id, err := builder.Reserve()
				if err != nil {
					t.Fatalf("Reserve() error = %v", err)
				}
				err = builder.Complete(id, atomicFacetType(test.primitive, facets))
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

func TestOrderedMixedTimezoneBoundsDefiniteOrder(t *testing.T) {
	t.Parallel()
	cases := [4]struct {
		name, bound, accepted, rejected, overlap, message string
		lower                                             bool
	}{
		{"zoned lower", "2000-01-15T12:00:00Z", "2000-01-16T03:00:00", "2000-01-14T21:00:00", "2000-01-15T12:00:00", "minInclusive", true},
		{"zoned upper", "2000-01-15T12:00:00Z", "2000-01-14T21:00:00", "2000-01-16T03:00:00", "2000-01-15T12:00:00", "maxInclusive", false},
		{"unzoned lower", "2000-01-15T12:00:00", "2000-01-16T03:00:00Z", "2000-01-14T21:00:00Z", "2000-01-15T12:00:00Z", "minInclusive", true},
		{"unzoned upper", "2000-01-15T12:00:00", "2000-01-14T21:00:00Z", "2000-01-16T03:00:00Z", "2000-01-15T12:00:00Z", "maxInclusive", false},
	}
	builder := value.NewBuilder(value.BuilderOptions{})
	var ids [4]value.TypeID
	for i, test := range cases {
		bound := value.BoundFacet{Lexical: test.bound, Type: value.NoType, Present: true}
		var facets value.FacetSpec
		if test.lower {
			facets.MinInclusive = bound
		} else {
			facets.MaxInclusive = bound
		}
		id, err := builder.Reserve()
		if err != nil {
			t.Fatal(err)
		}
		if completionErr := builder.Complete(id, atomicFacetType(value.PrimitiveDateTime, facets)); completionErr != nil {
			t.Fatal(completionErr)
		}
		ids[i] = id
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for i, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var scratch value.Scratch
			checkAccepted := func() {
				t.Helper()
				got, validationErr := program.Validate(ids[i], test.accepted, value.Resolver{}, 0, 16<<20, &scratch)
				if validationErr != nil {
					t.Fatalf("definite accepted %q: %v", test.accepted, validationErr)
				}
				if got.Type() != ids[i] || got.SelectedType() != value.NoType {
					t.Fatalf("accepted owner/selection = %v/%v, want %v/%v", got.Type(), got.SelectedType(), ids[i], value.NoType)
				}
			}
			checkAccepted()
			for _, input := range [2]string{test.rejected, test.overlap} {
				_, validationErr := program.Validate(ids[i], input, value.Resolver{}, 0, 16<<20, &scratch)
				if !errors.Is(validationErr, value.ErrFacet) || !strings.Contains(validationErr.Error(), test.message) {
					t.Fatalf("rejected %q: %v, want ErrFacet with %q", input, validationErr, test.message)
				}
				checkAccepted()
			}
		})
	}
}
