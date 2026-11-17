package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

var durationBoundEqualityCases = []struct {
	name, base, derived string
	equal               bool
}{
	{name: "years spelled as months", base: "P1Y", derived: "P12M", equal: true},
	{name: "a day spelled as hours", base: "P1D", derived: "PT24H", equal: true},
	{name: "years equal to days only in order", base: "P146097D", derived: "P400Y"},
	{name: "days equal to years only in order", base: "P400Y", derived: "P146097D"},
	{name: "incomparable month and days", base: "P1M", derived: "P30D"},
}

var orderedBounds = []struct {
	name string
	flag value.FacetMask
}{
	{"minInclusive", value.FacetMinInclusive},
	{"maxInclusive", value.FacetMaxInclusive},
	{"minExclusive", value.FacetMinExclusive},
	{"maxExclusive", value.FacetMaxExclusive},
}

func TestDurationExclusiveRestatementRequiresIdenticalValue(t *testing.T) {
	t.Parallel()
	for _, bound := range orderedBounds[2:] {
		facet := bound.flag
		for _, tc := range durationBoundEqualityCases {
			t.Run(bound.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
				base, err := builder.Reserve()
				if err != nil {
					t.Fatal(err)
				}
				if err = builder.Complete(base, atomicFacetType(value.PrimitiveDuration, orderedFacet(facet, tc.base, false))); err != nil {
					t.Fatal(err)
				}
				err = builder.AdmitBoundLiteral(base, facet, tc.derived)
				if tc.equal && err != nil {
					t.Fatalf("AdmitBoundLiteral(%q) restating %q = %v, want nil", tc.derived, tc.base, err)
				}
				if !tc.equal && !errors.Is(err, value.ErrFacet) {
					t.Fatalf("AdmitBoundLiteral(%q) against %q = %v, want ErrFacet", tc.derived, tc.base, err)
				}
			})
		}
	}
}

func TestDurationFixedBoundRequiresIdenticalValue(t *testing.T) {
	t.Parallel()
	for _, bound := range orderedBounds {
		facet := bound.flag
		for _, tc := range durationBoundEqualityCases {
			t.Run(bound.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
				base, err := builder.Reserve()
				if err != nil {
					t.Fatal(err)
				}
				if err = builder.Complete(base, atomicFacetType(value.PrimitiveDuration, orderedFacet(facet, tc.base, true))); err != nil {
					t.Fatal(err)
				}
				id, err := builder.Reserve()
				if err != nil {
					t.Fatal(err)
				}
				spec := atomicFacetType(value.PrimitiveDuration, orderedFacet(facet, tc.derived, false))
				spec.Base = base
				err = builder.Complete(id, spec)
				if tc.equal {
					if err != nil {
						t.Fatalf("Complete restating fixed %s with %s = %v, want nil", tc.base, tc.derived, err)
					}
					return
				}
				if !errors.Is(err, value.ErrFacet) {
					t.Fatalf("Complete changing fixed %s to %s = %v, want ErrFacet", tc.base, tc.derived, err)
				}
			})
		}
	}
}
