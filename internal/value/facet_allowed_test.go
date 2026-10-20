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
			name:      "atomic boolean enumeration disallowed",
			variety:   value.Atomic,
			primitive: value.PrimitiveBoolean,
			facet:     value.FacetEnumeration,
		},
		{
			name:      "atomic boolean whitespace",
			variety:   value.Atomic,
			primitive: value.PrimitiveBoolean,
			facet:     value.FacetWhiteSpace,
			want:      true,
		},
		{
			name:      "list boolean enumeration",
			variety:   value.List,
			primitive: value.PrimitiveBoolean,
			facet:     value.FacetEnumeration,
			want:      true,
		},
		{
			name:      "union boolean enumeration",
			variety:   value.Union,
			primitive: value.PrimitiveBoolean,
			facet:     value.FacetEnumeration,
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

func TestOrderedFacetApplicabilityContract(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		kind value.PrimitiveKind
		want bool
	}{
		{"string", value.PrimitiveString, false},
		{"boolean", value.PrimitiveBoolean, false},
		{"decimal", value.PrimitiveDecimal, true},
		{"float", value.PrimitiveFloat, true},
		{"double", value.PrimitiveDouble, true},
		{"duration", value.PrimitiveDuration, true},
		{"dateTime", value.PrimitiveDateTime, true},
		{"time", value.PrimitiveTime, true},
		{"date", value.PrimitiveDate, true},
		{"gYearMonth", value.PrimitiveGYearMonth, true},
		{"gYear", value.PrimitiveGYear, true},
		{"gMonthDay", value.PrimitiveGMonthDay, true},
		{"gDay", value.PrimitiveGDay, true},
		{"gMonth", value.PrimitiveGMonth, true},
		{"hexBinary", value.PrimitiveHexBinary, false},
		{"base64Binary", value.PrimitiveBase64Binary, false},
		{"anyURI", value.PrimitiveAnyURI, false},
		{"QName", value.PrimitiveQName, false},
		{"NOTATION", value.PrimitiveNotation, false},
		{"invalid next", value.PrimitiveKind(19), false},
		{"invalid maximum", value.PrimitiveKind(255), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, facet := range []value.FacetMask{value.FacetMinInclusive, value.FacetMinExclusive, value.FacetMaxInclusive, value.FacetMaxExclusive} {
				if got := value.FacetAllowed(value.Atomic, tc.kind, facet); got != tc.want {
					t.Fatalf("FacetAllowed(atomic, %d, %d) = %t, want %t", tc.kind, facet, got, tc.want)
				}
				for _, variety := range []value.Variety{value.List, value.Union, value.Variety(255)} {
					if value.FacetAllowed(variety, tc.kind, facet) {
						t.Fatalf("FacetAllowed(%d, %d, %d) accepted a non-atomic ordered facet", variety, tc.kind, facet)
					}
				}
			}
		})
	}
}
