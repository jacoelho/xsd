package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestFacetPresenceAdmissionPriority(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		primitive   value.PrimitiveKind
		facets      value.FacetSpec
		wantCause   error
		wantMessage string
	}{
		{
			name:      "missing enumeration before conflicting bounds",
			primitive: value.PrimitiveDecimal,
			facets: value.FacetSpec{
				Present:      value.FacetEnumeration,
				MinInclusive: value.BoundFacet{Present: true, Lexical: "1", Type: value.NoType},
				MinExclusive: value.BoundFacet{Present: true, Lexical: "2", Type: value.NoType},
			},
			wantCause: value.ErrMetadata,
		},
		{
			name:      "missing pattern before conflicting bounds",
			primitive: value.PrimitiveDecimal,
			facets: value.FacetSpec{
				Present:      value.FacetPattern,
				MinInclusive: value.BoundFacet{Present: true, Lexical: "1", Type: value.NoType},
				MinExclusive: value.BoundFacet{Present: true, Lexical: "2", Type: value.NoType},
			},
			wantCause: value.ErrMetadata,
		},
		{
			name:      "missing bound before conflicting bound flags",
			primitive: value.PrimitiveDecimal,
			facets: value.FacetSpec{
				Present:      value.FacetMinInclusive | value.FacetMinExclusive,
				MinInclusive: value.BoundFacet{Present: true, Lexical: "1", Type: value.NoType},
			},
			wantCause: value.ErrMetadata,
		},
		{
			name:        "digit applicability before missing digit payload",
			primitive:   value.PrimitiveString,
			facets:      value.FacetSpec{Present: value.FacetTotalDigits},
			wantCause:   value.ErrFacet,
			wantMessage: "digit facets require atomic decimal type",
		},
		{
			name:      "missing digit payload before unknown flag",
			primitive: value.PrimitiveDecimal,
			facets:    value.FacetSpec{Present: value.FacetTotalDigits | value.FacetMask(1<<15)},
			wantCause: value.ErrMetadata,
		},
		{
			name:        "unknown flag",
			primitive:   value.PrimitiveDecimal,
			facets:      value.FacetSpec{Present: value.FacetMask(1 << 15)},
			wantCause:   value.ErrFacet,
			wantMessage: "facet is not allowed for simple type",
		},
		{
			name:      "fixed whitespace infers presence",
			primitive: value.PrimitiveString,
			facets:    value.FacetSpec{Fixed: value.FacetWhiteSpace},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			err = builder.Complete(id, atomicFacetType(test.primitive, test.facets))
			if test.wantCause != nil {
				if !errors.Is(err, test.wantCause) {
					t.Fatalf("Complete() = %v; want %v", err, test.wantCause)
				}
				if test.wantMessage != "" && err.Error() != test.wantMessage {
					t.Fatalf("Complete() message = %q; want %q", err.Error(), test.wantMessage)
				}
				return
			}
			if err != nil {
				t.Fatalf("Complete() = %v", err)
			}
			if _, sealErr := builder.Seal(); sealErr != nil {
				t.Fatalf("Seal() = %v", sealErr)
			}
		})
	}
}
