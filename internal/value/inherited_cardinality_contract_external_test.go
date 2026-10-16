package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestInheritedCardinalityConflictsAndRetry(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		primitive value.PrimitiveKind
		base      value.FacetSpec
		own       value.FacetSpec
		wantError string
		retry     bool
	}{
		{
			name:      "inherited length below local minimum",
			primitive: value.PrimitiveString,
			base:      value.FacetSpec{Length: value.CardinalityFacet{Value: 3, Present: true}},
			own:       value.FacetSpec{MinLength: value.CardinalityFacet{Value: 4, Present: true}},
			wantError: "length cannot be less than minLength",
		},
		{
			name:      "local length above inherited maximum",
			primitive: value.PrimitiveString,
			base:      value.FacetSpec{MaxLength: value.CardinalityFacet{Value: 2, Present: true}},
			own:       value.FacetSpec{Length: value.CardinalityFacet{Value: 3, Present: true}},
			wantError: "length cannot exceed maxLength",
		},
		{
			name:      "inherited minimum above local maximum",
			primitive: value.PrimitiveString,
			base:      value.FacetSpec{MinLength: value.CardinalityFacet{Value: 3, Present: true}},
			own:       value.FacetSpec{MaxLength: value.CardinalityFacet{Value: 2, Present: true}},
			wantError: "minLength cannot exceed maxLength",
			retry:     true,
		},
		{
			name:      "local fraction digits above inherited total",
			primitive: value.PrimitiveDecimal,
			base:      value.FacetSpec{TotalDigits: value.CardinalityFacet{Value: 3, Present: true}},
			own:       value.FacetSpec{FractionDigits: value.CardinalityFacet{Value: 4, Present: true}},
			wantError: "fractionDigits cannot exceed totalDigits",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			builder := value.NewBuilder(value.BuilderOptions{})
			baseID, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if completionErr := builder.Complete(baseID, atomicFacetType(test.primitive, test.base)); completionErr != nil {
				t.Fatal(completionErr)
			}
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			spec := atomicFacetType(test.primitive, test.own)
			spec.Base = baseID
			err = builder.Complete(id, spec)
			if !errors.Is(err, value.ErrFacet) || err.Error() != test.wantError {
				t.Fatalf("Complete() = %v; want ErrFacet with %q", err, test.wantError)
			}
			if _, available := builder.InputRequirements(id); available {
				t.Fatal("failed type is available")
			}
			if !test.retry {
				return
			}

			spec.Facets = value.FacetSpec{
				MinLength: value.CardinalityFacet{Value: 999, Present: false},
				MaxLength: value.CardinalityFacet{Value: 5, Present: true},
			}
			if retryErr := builder.Complete(id, spec); retryErr != nil {
				t.Fatalf("Complete() retry = %v", retryErr)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			for _, lexical := range []string{"abc", "abcde"} {
				if _, err := program.Validate(id, lexical, value.Resolver{}, 0, 16<<20, nil); err != nil {
					t.Fatalf("Validate(%q) = %v", lexical, err)
				}
			}
			for _, lexical := range []string{"ab", "abcdef"} {
				if _, err := program.Validate(id, lexical, value.Resolver{}, 0, 16<<20, nil); !errors.Is(err, value.ErrFacet) {
					t.Fatalf("Validate(%q) = %v; want ErrFacet", lexical, err)
				}
			}
		})
	}
}
