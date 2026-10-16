package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestFacetAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		variety   value.Variety
		primitive value.PrimitiveKind
		facet     value.FacetMask
		want      bool
	}{
		{
			name:      "atomic string length",
			variety:   value.Atomic,
			primitive: value.PrimitiveString,
			facet:     value.FacetLength,
			want:      true,
		},
		{
			name:      "atomic string order disallowed",
			variety:   value.Atomic,
			primitive: value.PrimitiveString,
			facet:     value.FacetMinInclusive,
		},
		{
			name:      "atomic decimal digits",
			variety:   value.Atomic,
			primitive: value.PrimitiveDecimal,
			facet:     value.FacetTotalDigits,
			want:      true,
		},
		{
			name:      "atomic decimal length disallowed",
			variety:   value.Atomic,
			primitive: value.PrimitiveDecimal,
			facet:     value.FacetLength,
		},
		{
			name:      "atomic date order",
			variety:   value.Atomic,
			primitive: value.PrimitiveDate,
			facet:     value.FacetMaxExclusive,
			want:      true,
		},
		{
			name:      "atomic boolean common facet",
			variety:   value.Atomic,
			primitive: value.PrimitiveBoolean,
			facet:     value.FacetPattern,
			want:      true,
		},
		{
			name:      "list length",
			variety:   value.List,
			primitive: value.PrimitiveString,
			facet:     value.FacetMinLength,
			want:      true,
		},
		{
			name:      "list order disallowed",
			variety:   value.List,
			primitive: value.PrimitiveString,
			facet:     value.FacetMinInclusive,
		},
		{
			name:      "list whitespace",
			variety:   value.List,
			primitive: value.PrimitiveString,
			facet:     value.FacetWhiteSpace,
			want:      true,
		},
		{
			name:      "union enumeration",
			variety:   value.Union,
			primitive: value.PrimitiveString,
			facet:     value.FacetEnumeration,
			want:      true,
		},
		{
			name:      "union whitespace disallowed",
			variety:   value.Union,
			primitive: value.PrimitiveString,
			facet:     value.FacetWhiteSpace,
		},
		{
			name:      "zero facet mask",
			variety:   value.Atomic,
			primitive: value.PrimitiveString,
		},
		{
			name:      "unknown facet bit",
			variety:   value.Atomic,
			primitive: value.PrimitiveString,
			facet:     value.FacetMask(1 << 15),
		},
		{
			name:      "unknown atomic primitive pattern",
			variety:   value.Atomic,
			primitive: value.PrimitiveKind(255),
			facet:     value.FacetPattern,
			want:      true,
		},
		{
			name:      "unknown atomic primitive enumeration",
			variety:   value.Atomic,
			primitive: value.PrimitiveKind(255),
			facet:     value.FacetEnumeration,
			want:      true,
		},
		{
			name:      "unknown atomic primitive whitespace",
			variety:   value.Atomic,
			primitive: value.PrimitiveKind(255),
			facet:     value.FacetWhiteSpace,
			want:      true,
		},
		{
			name:      "unknown atomic primitive length",
			variety:   value.Atomic,
			primitive: value.PrimitiveKind(255),
			facet:     value.FacetLength,
		},
		{
			name:      "list length ignores primitive",
			variety:   value.List,
			primitive: value.PrimitiveKind(255),
			facet:     value.FacetLength,
			want:      true,
		},
		{
			name:      "union enumeration ignores primitive",
			variety:   value.Union,
			primitive: value.PrimitiveKind(255),
			facet:     value.FacetEnumeration,
			want:      true,
		},
		{
			name:      "invalid facet mask",
			variety:   value.Atomic,
			primitive: value.PrimitiveString,
			facet:     value.FacetLength | value.FacetPattern,
		},
		{
			name:      "invalid variety",
			variety:   value.Variety(99),
			primitive: value.PrimitiveString,
			facet:     value.FacetPattern,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := value.FacetAllowed(test.variety, test.primitive, test.facet)
			if got != test.want {
				t.Fatalf("FacetAllowed(%d, %d, %d) = %v, want %v", test.variety, test.primitive, test.facet, got, test.want)
			}
		})
	}
}
