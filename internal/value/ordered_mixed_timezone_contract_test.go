package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestOrderedMixedTimezoneBoundsDefiniteOrder(t *testing.T) {
	t.Parallel()
	cases := [4]struct {
		name, bound, accepted, rejected, overlap, message string
		lower                                             bool
	}{
		{"zoned lower", "2000-01-15T12:00:00Z", "2000-01-16T03:00:00", "2000-01-14T21:00:00", "2000-01-15T12:00:00", "lower bound facet failed", true},
		{"zoned upper", "2000-01-15T12:00:00Z", "2000-01-14T21:00:00", "2000-01-16T03:00:00", "2000-01-15T12:00:00", "upper bound facet failed", false},
		{"unzoned lower", "2000-01-15T12:00:00", "2000-01-16T03:00:00Z", "2000-01-14T21:00:00Z", "2000-01-15T12:00:00Z", "lower bound facet failed", true},
		{"unzoned upper", "2000-01-15T12:00:00", "2000-01-14T21:00:00Z", "2000-01-16T03:00:00Z", "2000-01-15T12:00:00Z", "upper bound facet failed", false},
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
				if !errors.Is(validationErr, value.ErrFacet) || validationErr.Error() != test.message {
					t.Fatalf("rejected %q: %v, want ErrFacet with %q", input, validationErr, test.message)
				}
				checkAccepted()
			}
		})
	}
}
