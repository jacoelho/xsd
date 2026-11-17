package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestTimeFacetDerivationRejectsWideningAndCanRetry(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, base, derived, accepted string }{
		{"adjusted timezone", "12:00:00-10:00", "12:00:00-14:00", "00:00:00Z"},
		{"incomparable timezone", "12:00:00", "12:00:00Z", "11:00:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
			base, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if err = builder.Complete(base, atomicFacetType(value.PrimitiveTime, value.FacetSpec{MaxInclusive: value.BoundFacet{Lexical: tc.base, Type: value.NoType, Present: true}})); err != nil {
				t.Fatal(err)
			}
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			spec := atomicFacetType(value.PrimitiveTime, value.FacetSpec{MaxInclusive: value.BoundFacet{Lexical: tc.derived, Type: value.NoType, Present: true}})
			spec.Base = base
			if err = builder.Complete(id, spec); !errors.Is(err, value.ErrFacet) {
				t.Fatalf("Complete widening bound = %v, want ErrFacet", err)
			}
			spec.Facets = value.FacetSpec{}
			if err = builder.Complete(id, spec); err != nil {
				t.Fatalf("Complete after removing invalid bound: %v", err)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := program.Validate(id, tc.accepted, value.Resolver{}, 0, value.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil); err != nil {
				t.Fatalf("Validate after retry: %v", err)
			}
		})
	}
}

func TestTimeFacetDerivationAllowsContainedUnknownTimezoneBound(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	base, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(base, atomicFacetType(value.PrimitiveTime, value.FacetSpec{
		MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "22:00:00Z", Type: value.NoType},
			Present:     true,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	reservedID, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatalf("Reserve() error = %v", reserveErr)
	}
	err = builder.Complete(reservedID, value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveTime,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              base,
		ListItem:          value.NoType,
		Facets: value.FacetSpec{MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "00:00:00", Type: value.NoType},
			Present:     true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("contained unknown-timezone bound rejected: %v", err)
	}
}

func TestTimeFacetRuntimeComparisonUsesRecurringTimeOfDay(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	typeID, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(typeID, atomicFacetType(value.PrimitiveTime, value.FacetSpec{
		MinInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "10:30:00Z", Type: value.NoType},
			Present:     true,
		},
		MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "10:30:00Z", Type: value.NoType},
			Present:     true,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(typeID, "00:30:00+14:00", value.Resolver{}, 0, value.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil); err != nil {
		t.Fatalf("timezone-adjusted recurring time violated equal bounds: %v", err)
	}
}

func TestTemporalNormalizedFractionOrdering(t *testing.T) {
	for _, test := range []struct {
		name, left, right string
		want              value.OrderedFacetRelation
	}{
		{"trailing zeros", ".1200", ".12", value.OrderedFacetEqual},
		{"strict prefix", ".12", ".120001", value.OrderedFacetLess},
		{"zero fraction", ".0000", "", value.OrderedFacetEqual},
	} {
		t.Run(test.name, func(t *testing.T) {
			leftTime, err := value.ParseTimeValue("12:00:00" + test.left)
			if err != nil {
				t.Fatal(err)
			}
			rightTime, err := value.ParseTimeValue("12:00:00" + test.right)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.CompareTimePartial(leftTime, rightTime); got != test.want {
				t.Fatalf("CompareTimePartial() = %v, want %v", got, test.want)
			}
			leftDateTime, err := value.ParseDateTimeValue("2000-01-01T12:00:00" + test.left)
			if err != nil {
				t.Fatal(err)
			}
			rightDateTime, err := value.ParseDateTimeValue("2000-01-01T12:00:00" + test.right)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.CompareTemporalValues(leftDateTime.Temporal(), rightDateTime.Temporal()); got != test.want {
				t.Fatalf("CompareTemporalValues() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTimeExclusiveRestatementIsAdmittedByClockAndDerivedByDay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		facet         value.FacetMask
		base, derived string
		admitted      bool
		derives       bool
	}{
		{"same next-day spelling", value.FacetMaxExclusive, "23:00:00-02:00", "23:00:00-02:00", true, true},
		{"same previous-day spelling", value.FacetMinExclusive, "01:00:00+02:00", "01:00:00+02:00", true, true},
		{"upper same clock on earlier day", value.FacetMaxExclusive, "23:00:00-02:00", "01:00:00Z", true, true},
		{"lower same clock on later day", value.FacetMinExclusive, "01:00:00+02:00", "23:00:00Z", true, true},
		{"upper same clock on later day", value.FacetMaxExclusive, "01:00:00Z", "23:00:00-02:00", true, false},
		{"upper same clock on later day from previous-day base", value.FacetMaxExclusive, "00:00:00+01:00", "23:00:00Z", true, false},
		{"upper later clock", value.FacetMaxExclusive, "01:00:00Z", "01:00:01Z", false, false},
		{"upper midnight restated at start of day", value.FacetMaxExclusive, "24:00:00", "00:00:00", true, true},
		{"lower start of day restated at midnight", value.FacetMinExclusive, "00:00:00", "24:00:00", true, true},
		{"upper start of day restated at midnight", value.FacetMaxExclusive, "00:00:00", "24:00:00", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
			base, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if err = builder.Complete(base, atomicFacetType(value.PrimitiveTime, exclusiveFacet(tc.facet, tc.base))); err != nil {
				t.Fatal(err)
			}
			err = builder.AdmitBoundLiteral(base, tc.facet, tc.derived)
			if !tc.admitted {
				if !errors.Is(err, value.ErrFacet) {
					t.Fatalf("AdmitBoundLiteral(%q) = %v, want ErrFacet", tc.derived, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("AdmitBoundLiteral(%q) = %v, want nil", tc.derived, err)
			}
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			spec := atomicFacetType(value.PrimitiveTime, exclusiveFacet(tc.facet, tc.derived))
			spec.Base = base
			err = builder.Complete(id, spec)
			if tc.derives && err != nil {
				t.Fatalf("Complete(%q restricting %q) = %v, want nil", tc.derived, tc.base, err)
			}
			if !tc.derives && !errors.Is(err, value.ErrFacet) {
				t.Fatalf("Complete(%q restricting %q) = %v, want ErrFacet", tc.derived, tc.base, err)
			}
		})
	}
}

func exclusiveFacet(facet value.FacetMask, lexical string) value.FacetSpec {
	bound := value.BoundFacet{Lexical: lexical, Type: value.NoType, Present: true}
	if facet == value.FacetMinExclusive {
		return value.FacetSpec{MinExclusive: bound}
	}
	return value.FacetSpec{MaxExclusive: bound}
}
