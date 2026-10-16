package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestProgramLexicalPayloadLengthFacets(t *testing.T) {
	for _, tc := range []struct {
		name      string
		primitive value.PrimitiveKind
		length    uint32
		accepted  string
		canonical string
		rejected  []string
	}{
		{"URI characters", value.PrimitiveAnyURI, 2, "éx", "éx", []string{"é", "éxy"}},
		{"base64 octets", value.PrimitiveBase64Binary, 1, " Y Q == ", "YQ==", []string{"", "YWI="}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{})
			id, reserveErr := builder.Reserve()
			if reserveErr != nil {
				t.Fatal(reserveErr)
			}
			if completeErr := builder.Complete(id, value.TypeSpec{
				Variety:           value.Atomic,
				Primitive:         tc.primitive,
				Whitespace:        value.WhitespaceCollapse,
				WhitespacePresent: true,
				Base:              value.NoType,
				ListItem:          value.NoType,
				Facets: value.FacetSpec{
					Length: value.CardinalityFacet{Value: tc.length, Present: true},
				},
			}); completeErr != nil {
				t.Fatal(completeErr)
			}
			program, sealErr := builder.Seal()
			if sealErr != nil {
				t.Fatal(sealErr)
			}
			for _, form := range []string{"string", "bytes"} {
				for _, needs := range []value.Needs{0, value.NeedCanonical} {
					validate := func(input string) (value.Value, error) {
						if form == "bytes" {
							return program.ValidateBytes(id, []byte(input), value.Resolver{}, needs, 16<<20, nil)
						}
						return program.Validate(id, input, value.Resolver{}, needs, 16<<20, nil)
					}
					accepted, validationErr := validate(tc.accepted)
					if validationErr != nil {
						t.Fatalf("%s needs=%d accepted input: %v", form, needs, validationErr)
					}
					wantCanonical := ""
					if needs == value.NeedCanonical {
						wantCanonical = tc.canonical
					}
					if accepted.CanonicalText() != wantCanonical {
						t.Fatalf("%s needs=%d canonical = %q, want %q", form, needs, accepted.CanonicalText(), wantCanonical)
					}
					for _, input := range tc.rejected {
						if _, rejectedErr := validate(input); !errors.Is(rejectedErr, value.ErrFacet) || rejectedErr.Error() != "length facet failed" {
							t.Fatalf("%s needs=%d input=%q error = %v, want length facet failed", form, needs, input, rejectedErr)
						}
					}
				}
			}
		})
	}
}

func TestBinaryParserRejectsMisplacedPaddingAndInvalidAlphabet(t *testing.T) {
	for _, input := range []string{"Y=Q=", "A@=="} {
		for _, needs := range []value.PrimitiveValueNeed{0, value.PrimitiveNeedCanonical} {
			got, err := value.ParseBinaryValue(value.PrimitiveBase64Binary, input, needs)
			if err == nil || err.Error() != "invalid base64Binary" {
				t.Fatalf("input=%q needs=%d error = %v, want invalid base64Binary", input, needs, err)
			}
			if got != (value.BinaryValue{}) {
				t.Fatalf("input=%q needs=%d value = %#v, want zero after failure", input, needs, got)
			}
		}
	}
}
