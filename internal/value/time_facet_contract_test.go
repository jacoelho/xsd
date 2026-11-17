package value_test

import (
	"errors"
	"strings"
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

func TestTimeBoundDiagnosticNamesTheBoundValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, bound, lexical, diagnosis string
		facet                           value.FacetMask
	}{
		{name: "bound spelled on the next day", facet: value.FacetMaxInclusive, bound: "23:00:00-02:00", lexical: "01:00:01Z",
			diagnosis: `value "01:00:01Z" does not satisfy maxInclusive "01:00:00Z"`},
		{name: "bound spelled on the previous day", facet: value.FacetMinInclusive, bound: "00:30:00+14:00", lexical: "10:29:59Z",
			diagnosis: `value "10:29:59Z" does not satisfy minInclusive "10:30:00Z"`},
		{name: "value spelled on the next day", facet: value.FacetMaxExclusive, bound: "01:00:00Z", lexical: "23:00:00-02:00",
			diagnosis: `value "23:00:00-02:00" does not satisfy maxExclusive "01:00:00Z"`},
		{name: "end-of-day bound", facet: value.FacetMaxExclusive, bound: "24:00:00", lexical: "24:00:00",
			diagnosis: `value "24:00:00" does not satisfy maxExclusive "24:00:00"`},
		{name: "end-of-day bound with a zero offset", facet: value.FacetMinInclusive, bound: "24:00:00+00:00", lexical: "00:00:00Z",
			diagnosis: `value "00:00:00Z" does not satisfy minInclusive "24:00:00Z"`},
		{name: "end-of-day value", facet: value.FacetMaxInclusive, bound: "23:59:59Z", lexical: "24:00:00Z",
			diagnosis: `value "24:00:00Z" does not satisfy maxInclusive "23:59:59Z"`},
		{name: "indeterminate against a timezoned bound", facet: value.FacetMaxInclusive, bound: "12:00:00Z", lexical: "12:00:00",
			diagnosis: `value "12:00:00" does not satisfy maxInclusive "12:00:00Z"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if err = builder.Complete(id, atomicFacetType(value.PrimitiveTime, orderedFacet(tc.facet, tc.bound, false))); err != nil {
				t.Fatal(err)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			_, err = program.Validate(id, tc.lexical, value.Resolver{}, 0, value.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
			if !errors.Is(err, value.ErrFacet) || err.Error() != tc.diagnosis {
				t.Fatalf("Validate(%q) = %v, want ErrFacet %q", tc.lexical, err, tc.diagnosis)
			}
		})
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

// timeRestrictionCase restricts a base holding one bound by a derived bound.
// admitDiagnosis is the AdmitBoundLiteral failure, or empty when the literal
// is admitted; completeFailure is the reason Complete then rejects the derived
// type, or empty when it accepts it.
type timeRestrictionCase struct {
	name                    string
	baseFacet, derivedFacet value.FacetMask
	base, derived           string
	admitDiagnosis          string
	completeFailure         string
}

const (
	upperLoosensBase = "upper ordered facet cannot loosen base restriction"
	lowerLoosensBase = "lower ordered facet cannot loosen base restriction"
	fixedBoundChange = "fixed ordered facet cannot change"
)

func runTimeRestrictionCases(t *testing.T, fixedBase bool, cases []timeRestrictionCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
			base, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if err = builder.Complete(base, atomicFacetType(value.PrimitiveTime, orderedFacet(tc.baseFacet, tc.base, fixedBase))); err != nil {
				t.Fatal(err)
			}
			err = builder.AdmitBoundLiteral(base, tc.derivedFacet, tc.derived)
			if tc.admitDiagnosis != "" {
				if !errors.Is(err, value.ErrFacet) || err.Error() != tc.admitDiagnosis {
					t.Fatalf("AdmitBoundLiteral(%q) = %v, want ErrFacet %q", tc.derived, err, tc.admitDiagnosis)
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
			spec := atomicFacetType(value.PrimitiveTime, orderedFacet(tc.derivedFacet, tc.derived, false))
			spec.Base = base
			err = builder.Complete(id, spec)
			if tc.completeFailure == "" {
				if err != nil {
					t.Fatalf("Complete(%q restricting %q) = %v, want nil", tc.derived, tc.base, err)
				}
				return
			}
			if !errors.Is(err, value.ErrFacet) || !strings.Contains(err.Error(), tc.completeFailure) {
				t.Fatalf("Complete(%q restricting %q) = %v, want ErrFacet %q", tc.derived, tc.base, err, tc.completeFailure)
			}
		})
	}
}

func TestTimeExclusiveRestatementEqualsByValueAndRestrictsBySameSideDay(t *testing.T) {
	t.Parallel()
	upper, lower := value.FacetMaxExclusive, value.FacetMinExclusive
	runTimeRestrictionCases(t, false, []timeRestrictionCase{
		{name: "upper same next-day spelling", baseFacet: upper, derivedFacet: upper, base: "23:00:00-02:00", derived: "23:00:00-02:00"},
		{name: "lower same previous-day spelling", baseFacet: lower, derivedFacet: lower, base: "01:00:00+02:00", derived: "01:00:00+02:00"},
		{name: "upper equal value spelled a day earlier", baseFacet: upper, derivedFacet: upper, base: "23:00:00-02:00", derived: "01:00:00Z"},
		{name: "lower equal value spelled a day later", baseFacet: lower, derivedFacet: lower, base: "01:00:00+02:00", derived: "23:00:00Z"},
		{name: "upper equal value spelled a day later", baseFacet: upper, derivedFacet: upper, base: "01:00:00Z", derived: "23:00:00-02:00",
			completeFailure: upperLoosensBase},
		{name: "upper equal value spelled a day later than a previous-day base", baseFacet: upper, derivedFacet: upper, base: "00:00:00+01:00", derived: "23:00:00Z",
			completeFailure: upperLoosensBase},
		{name: "lower equal value spelled a day earlier", baseFacet: lower, derivedFacet: lower, base: "23:00:00Z", derived: "01:00:00+02:00",
			completeFailure: lowerLoosensBase},
		{name: "upper later time", baseFacet: upper, derivedFacet: upper, base: "01:00:00Z", derived: "01:00:01Z",
			admitDiagnosis: `value "01:00:01Z" does not satisfy maxExclusive "01:00:00Z"`},
		{name: "upper end of day restated with a zero offset", baseFacet: upper, derivedFacet: upper, base: "24:00:00Z", derived: "24:00:00+00:00"},
		{name: "upper end of day tightened to start of day", baseFacet: upper, derivedFacet: upper, base: "24:00:00", derived: "00:00:00"},
		{name: "lower start of day tightened to end of day", baseFacet: lower, derivedFacet: lower, base: "00:00:00", derived: "24:00:00"},
		{name: "upper start of day is not restated by end of day", baseFacet: upper, derivedFacet: upper, base: "00:00:00", derived: "24:00:00",
			admitDiagnosis: `value "24:00:00" does not satisfy maxExclusive "00:00:00"`},
		{name: "lower end of day is not restated by start of day", baseFacet: lower, derivedFacet: lower, base: "24:00:00Z", derived: "00:00:00Z",
			admitDiagnosis: `value "00:00:00Z" does not satisfy minExclusive "24:00:00Z"`},
	})
}

func TestTimeBoundLiteralIsAdmittedByValueAndRestrictsBySameSideDay(t *testing.T) {
	t.Parallel()
	maxExclusive, maxInclusive := value.FacetMaxExclusive, value.FacetMaxInclusive
	minExclusive, minInclusive := value.FacetMinExclusive, value.FacetMinInclusive
	runTimeRestrictionCases(t, false, []timeRestrictionCase{
		{name: "upper exclusive later time spelled a day earlier", baseFacet: maxExclusive, derivedFacet: maxExclusive, base: "23:00:00-02:00", derived: "02:00:00Z",
			admitDiagnosis: `value "02:00:00Z" does not satisfy maxExclusive "01:00:00Z"`},
		{name: "upper inclusive later time spelled a day earlier", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "23:00:00-02:00", derived: "02:00:00Z",
			admitDiagnosis: `value "02:00:00Z" does not satisfy maxInclusive "01:00:00Z"`},
		{name: "lower inclusive earlier time spelled a day later", baseFacet: minInclusive, derivedFacet: minInclusive, base: "01:00:00+02:00", derived: "22:00:00Z",
			admitDiagnosis: `value "22:00:00Z" does not satisfy minInclusive "23:00:00Z"`},
		{name: "lower exclusive earlier time spelled a day later", baseFacet: minExclusive, derivedFacet: minExclusive, base: "01:00:00+02:00", derived: "22:00:00Z",
			admitDiagnosis: `value "22:00:00Z" does not satisfy minExclusive "23:00:00Z"`},
		{name: "upper inclusive earlier time spelled a day earlier", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "23:00:00-02:00", derived: "00:30:00Z"},
		{name: "lower inclusive later time spelled a day later", baseFacet: minInclusive, derivedFacet: minInclusive, base: "01:00:00+02:00", derived: "23:30:00Z"},
		{name: "upper inclusive earlier time spelled a day later", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "00:00:00+01:00", derived: "22:00:00Z",
			completeFailure: upperLoosensBase},
		{name: "upper inclusive earlier time on a later day of one offset sign", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "12:00:00-10:00", derived: "12:00:00-14:00",
			completeFailure: upperLoosensBase},
		{name: "lower inclusive later time spelled a day earlier", baseFacet: minInclusive, derivedFacet: minInclusive, base: "23:00:00-01:00", derived: "02:00:00Z",
			completeFailure: lowerLoosensBase},
		{name: "upper against base lower by value across days", baseFacet: minInclusive, derivedFacet: maxInclusive, base: "23:00:00-02:00", derived: "02:00:00Z"},
		{name: "lower against base upper by value across days", baseFacet: maxInclusive, derivedFacet: minInclusive, base: "00:00:00+01:00", derived: "22:00:00Z"},
		{name: "upper below base lower by value", baseFacet: minInclusive, derivedFacet: maxInclusive, base: "01:00:00+02:00", derived: "22:30:00Z",
			admitDiagnosis: `value "22:30:00Z" does not satisfy minInclusive "23:00:00Z"`},
		{name: "lower above base upper by value", baseFacet: maxInclusive, derivedFacet: minInclusive, base: "23:00:00-02:00", derived: "02:00:00Z",
			admitDiagnosis: `value "02:00:00Z" does not satisfy maxInclusive "01:00:00Z"`},
	})
}

func TestTimeSameStepBoundsAreOrderedByValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, lower, upper string
		valid              bool
	}{
		{name: "lower spelled a day earlier is the later time", lower: "01:00:00+02:00", upper: "22:30:00Z"},
		{name: "lower spelled a day later is the earlier time", lower: "23:00:00-02:00", upper: "02:00:00Z", valid: true},
		{name: "equal values spelled on different days", lower: "23:00:00-02:00", upper: "01:00:00Z", valid: true},
		{name: "start of day to end of day", lower: "00:00:00Z", upper: "24:00:00Z", valid: true},
		{name: "end of day to start of day", lower: "24:00:00Z", upper: "00:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			err = builder.Complete(id, atomicFacetType(value.PrimitiveTime, value.FacetSpec{
				MinInclusive: value.BoundFacet{Lexical: tc.lower, Type: value.NoType, Present: true},
				MaxInclusive: value.BoundFacet{Lexical: tc.upper, Type: value.NoType, Present: true},
			}))
			if tc.valid && err != nil {
				t.Fatalf("Complete(minInclusive %q, maxInclusive %q) = %v, want nil", tc.lower, tc.upper, err)
			}
			if !tc.valid && !errors.Is(err, value.ErrFacet) {
				t.Fatalf("Complete(minInclusive %q, maxInclusive %q) = %v, want ErrFacet", tc.lower, tc.upper, err)
			}
		})
	}
}

func TestTimeFixedBoundIsKeptByAnEqualValueThatDoesNotLoosenIt(t *testing.T) {
	t.Parallel()
	maxExclusive, maxInclusive := value.FacetMaxExclusive, value.FacetMaxInclusive
	minExclusive, minInclusive := value.FacetMinExclusive, value.FacetMinInclusive
	runTimeRestrictionCases(t, true, []timeRestrictionCase{
		{name: "maxExclusive same instant spelled on the same day", baseFacet: maxExclusive, derivedFacet: maxExclusive, base: "23:00:00-02:00", derived: "22:00:00-03:00"},
		{name: "maxExclusive equal value spelled a day earlier", baseFacet: maxExclusive, derivedFacet: maxExclusive, base: "23:00:00-02:00", derived: "01:00:00Z"},
		{name: "maxInclusive same instant spelled on the same day", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "23:00:00-02:00", derived: "22:00:00-03:00"},
		{name: "maxInclusive equal value spelled a day earlier", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "23:00:00-02:00", derived: "01:00:00Z"},
		{name: "minExclusive same instant spelled on the same day", baseFacet: minExclusive, derivedFacet: minExclusive, base: "01:00:00+02:00", derived: "02:00:00+03:00"},
		{name: "minExclusive equal value spelled a day later", baseFacet: minExclusive, derivedFacet: minExclusive, base: "01:00:00+02:00", derived: "23:00:00Z"},
		{name: "minInclusive same instant spelled on the same day", baseFacet: minInclusive, derivedFacet: minInclusive, base: "01:00:00+02:00", derived: "02:00:00+03:00"},
		{name: "minInclusive equal value spelled a day later", baseFacet: minInclusive, derivedFacet: minInclusive, base: "01:00:00+02:00", derived: "23:00:00Z"},
		{name: "maxExclusive equal value spelled a day later", baseFacet: maxExclusive, derivedFacet: maxExclusive, base: "01:00:00Z", derived: "23:00:00-02:00",
			completeFailure: upperLoosensBase},
		{name: "maxInclusive equal value spelled a day later", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "01:00:00Z", derived: "23:00:00-02:00",
			completeFailure: upperLoosensBase},
		{name: "minExclusive equal value spelled a day earlier", baseFacet: minExclusive, derivedFacet: minExclusive, base: "23:00:00Z", derived: "01:00:00+02:00",
			completeFailure: lowerLoosensBase},
		{name: "minInclusive equal value spelled a day earlier", baseFacet: minInclusive, derivedFacet: minInclusive, base: "23:00:00Z", derived: "01:00:00+02:00",
			completeFailure: lowerLoosensBase},
		{name: "minInclusive start of day changed to end of day", baseFacet: minInclusive, derivedFacet: minInclusive, base: "00:00:00Z", derived: "24:00:00Z",
			completeFailure: fixedBoundChange},
		{name: "maxInclusive end of day changed to start of day", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "24:00:00Z", derived: "00:00:00Z",
			completeFailure: fixedBoundChange},
		{name: "maxInclusive end of day spelled with a zero offset", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "24:00:00Z", derived: "24:00:00-00:00"},
		{name: "maxInclusive without timezone spelled identically", baseFacet: maxInclusive, derivedFacet: maxInclusive, base: "12:00:00", derived: "12:00:00"},
		{name: "minExclusive without timezone spelled identically", baseFacet: minExclusive, derivedFacet: minExclusive, base: "12:00:00", derived: "12:00:00"},
	})
}
