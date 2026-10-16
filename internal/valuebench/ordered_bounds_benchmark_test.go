package valuebench

import (
	"errors"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

type admittedOrderedRow struct {
	id             valuepkg.TypeID
	input, message string
}

type admittedOrderedFixture struct {
	name    string
	program *valuepkg.Program
	rows    [6]admittedOrderedRow
}

func admittedOrderedSpec(primitive valuepkg.PrimitiveKind, base valuepkg.TypeID, facets valuepkg.FacetSpec) valuepkg.TypeSpec {
	return valuepkg.TypeSpec{Variety: valuepkg.Atomic, Primitive: primitive,
		Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
		Base: base, ListItem: valuepkg.NoType, Facets: facets}
}

//nolint:revive // Fixedness is facet input data in this fixture.
func admittedOrderedFacet(flag valuepkg.FacetMask, lexical string, fixed bool) valuepkg.FacetSpec {
	bound := valuepkg.BoundFacet{Lexical: lexical, Type: valuepkg.NoType, Present: true}
	var facets valuepkg.FacetSpec
	//nolint:exhaustive // These fixtures use the four ordered facet directions.
	switch flag {
	case valuepkg.FacetMinInclusive:
		facets.MinInclusive = bound
	case valuepkg.FacetMinExclusive:
		facets.MinExclusive = bound
	case valuepkg.FacetMaxInclusive:
		facets.MaxInclusive = bound
	case valuepkg.FacetMaxExclusive:
		facets.MaxExclusive = bound
	}
	if fixed {
		facets.Fixed = flag
	}
	return facets
}

//nolint:thelper // This helper runs inside measured construction without test-stack tracking.
func completeAdmittedOrdered(tb testing.TB, builder *valuepkg.Builder, spec valuepkg.TypeSpec) valuepkg.TypeID {
	id, err := builder.Reserve()
	if err != nil {
		tb.Fatal(err)
	}
	if err := builder.Complete(id, spec); err != nil {
		tb.Fatal(err)
	}
	return id
}

func admittedOrderedFixtures(tb testing.TB) [4]admittedOrderedFixture {
	tb.Helper()
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	inclusive := admittedOrderedFacet(valuepkg.FacetMinInclusive, "1.5", false)
	inclusive.MaxInclusive = admittedOrderedFacet(valuepkg.FacetMaxInclusive, "2.5", false).MaxInclusive
	inc := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, valuepkg.NoType, inclusive))
	exclusive := admittedOrderedFacet(valuepkg.FacetMinExclusive, "1.5", false)
	exclusive.MaxExclusive = admittedOrderedFacet(valuepkg.FacetMaxExclusive, "2.5", false).MaxExclusive
	exc := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, valuepkg.NoType, exclusive))
	timeLower := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDateTime, valuepkg.NoType, admittedOrderedFacet(valuepkg.FacetMinInclusive, "2000-01-01T12:00:00Z", false)))
	timeUpper := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDateTime, valuepkg.NoType, admittedOrderedFacet(valuepkg.FacetMaxInclusive, "2000-01-01T12:00:00Z", false)))
	durationLower := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDuration, valuepkg.NoType, admittedOrderedFacet(valuepkg.FacetMinInclusive, "P365D", false)))
	durationUpper := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDuration, valuepkg.NoType, admittedOrderedFacet(valuepkg.FacetMaxInclusive, "P365D", false)))
	program, err := builder.Seal()
	if err != nil {
		tb.Fatal(err)
	}
	return [4]admittedOrderedFixture{
		{"DecimalInclusive", program, [6]admittedOrderedRow{
			{inc, "1.5", ""}, {inc, "2", ""}, {inc, "2.5", ""},
			{inc, "1.49", "lower bound facet failed"}, {inc, "2.51", "upper bound facet failed"}, {inc, "0", "lower bound facet failed"},
		}},
		{"DecimalExclusive", program, [6]admittedOrderedRow{
			{exc, "1.5", "lower bound facet failed"}, {exc, "2", ""}, {exc, "2.5", "upper bound facet failed"},
			{exc, "1.49", "lower bound facet failed"}, {exc, "2.51", "upper bound facet failed"}, {exc, "2.00", ""},
		}},
		// TimePartial times dateTime's partial order; recurring time is checked outside the loop.
		{"TimePartial", program, [6]admittedOrderedRow{
			{timeLower, "2000-01-01T12:00:00Z", ""}, {timeUpper, "2000-01-01T12:00:00Z", ""},
			{timeLower, "2000-01-01T12:00:00", "lower bound facet failed"}, {timeUpper, "2000-01-01T12:00:00", "upper bound facet failed"},
			{timeLower, "2000-01-01T13:00:00Z", ""}, {timeUpper, "2000-01-01T11:00:00Z", ""},
		}},
		{"DurationPartial", program, [6]admittedOrderedRow{
			{durationLower, "P365D", ""}, {durationUpper, "P365D", ""},
			{durationLower, "P1Y", "lower bound facet failed"}, {durationUpper, "P1Y", "upper bound facet failed"},
			{durationLower, "P366D", ""}, {durationUpper, "P364D", ""},
		}},
	}
}

func checkAdmittedOrderedResult(tb testing.TB, got valuepkg.Value, id valuepkg.TypeID) {
	tb.Helper()
	if got.Type() != id || got.SelectedType() != valuepkg.NoType || got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
		tb.Fatalf("unprojected typed result = %#v", got)
	}
}

func checkAdmittedOrderedRow(tb testing.TB, program *valuepkg.Program, row admittedOrderedRow, scratch *valuepkg.Scratch) {
	tb.Helper()
	got, err := program.Validate(row.id, row.input, valuepkg.Resolver{}, 0, 16<<20, scratch)
	checkAdmittedOrderedOutcome(tb, got, err, row)
}

func checkAdmittedOrderedOutcome(tb testing.TB, got valuepkg.Value, err error, row admittedOrderedRow) {
	tb.Helper()
	if row.message != "" {
		if err == nil || err.Error() != row.message || !errors.Is(err, valuepkg.ErrFacet) {
			tb.Fatalf("Validate(%q) = %v, want ErrFacet with %q", row.input, err, row.message)
		}
		return
	}
	if err != nil {
		tb.Fatalf("Validate(%q) = %v", row.input, err)
	}
	checkAdmittedOrderedResult(tb, got, row.id)
}

func checkAdmittedOrderedFixture(tb testing.TB, fixture admittedOrderedFixture, scratch *valuepkg.Scratch) {
	tb.Helper()
	var recovery admittedOrderedRow
	for _, row := range fixture.rows {
		if row.message == "" {
			recovery = row
			break
		}
	}
	for _, row := range fixture.rows {
		checkAdmittedOrderedRow(tb, fixture.program, row, scratch)
		if row.message != "" {
			checkAdmittedOrderedRow(tb, fixture.program, recovery, scratch)
		}
	}
}

func BenchmarkAdmittedOrderedBounds(b *testing.B) {
	for _, fixture := range admittedOrderedFixtures(b) {
		b.Run(fixture.name, func(b *testing.B) {
			var scratch valuepkg.Scratch
			checkAdmittedOrderedFixture(b, fixture, &scratch)
			var outputs [6]valuepkg.Value
			var outcomes [6]error
			b.ReportAllocs()
			for b.Loop() {
				for i, row := range fixture.rows {
					outputs[i], outcomes[i] = fixture.program.Validate(row.id, row.input, valuepkg.Resolver{}, 0, 16<<20, &scratch)
					if (outcomes[i] != nil) != (row.message != "") {
						b.Fatalf("Validate(%q) unexpected error presence: %v", row.input, outcomes[i])
					}
				}
			}
			for i, row := range fixture.rows {
				checkAdmittedOrderedOutcome(b, outputs[i], outcomes[i], row)
			}
			checkAdmittedOrderedFixture(b, fixture, &scratch)
		})
	}
}

//nolint:thelper // This helper runs inside measured construction without test-stack tracking.
func checkAdmittedOrderedCompletionError(tb testing.TB, err error, message string) {
	if err == nil || err.Error() != message || !errors.Is(err, valuepkg.ErrFacet) {
		tb.Fatalf("Complete() = %v, want ErrFacet with %q", err, message)
	}
}

//nolint:revive,thelper // Widening selects the untimed opponent; test-stack tracking stays outside measurement.
func buildAdmittedOrderedBatch(tb testing.TB, widening bool) ([4]*valuepkg.Program, [4]valuepkg.TypeID) {
	var programs [4]*valuepkg.Program
	var leaves [4]valuepkg.TypeID
	for i, flag := range [4]valuepkg.FacetMask{valuepkg.FacetMinInclusive, valuepkg.FacetMinExclusive, valuepkg.FacetMaxInclusive, valuepkg.FacetMaxExclusive} {
		older, middle, changed, equivalent := "1", "2", "3", "2.0"
		message := "fixed ordered facet cannot change"
		if flag == valuepkg.FacetMaxInclusive || flag == valuepkg.FacetMaxExclusive {
			older, middle, changed, equivalent = "10", "9", "8", "9.00"
			if widening {
				changed, message = "11", "upper ordered facet cannot loosen base restriction"
			}
		} else if widening {
			changed, message = "0", "lower ordered facet cannot loosen base restriction"
		}
		builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
		base := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, valuepkg.NoType, admittedOrderedFacet(flag, older, false)))
		fixed := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, base, admittedOrderedFacet(flag, middle, true)))
		omitted := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveDecimal, fixed, valuepkg.FacetSpec{}))
		leaf, err := builder.Reserve()
		if err != nil {
			tb.Fatal(err)
		}
		err = builder.Complete(leaf, admittedOrderedSpec(valuepkg.PrimitiveDecimal, omitted, admittedOrderedFacet(flag, changed, false)))
		checkAdmittedOrderedCompletionError(tb, err, message)
		if retryErr := builder.Complete(leaf, admittedOrderedSpec(valuepkg.PrimitiveDecimal, omitted, admittedOrderedFacet(flag, equivalent, false))); retryErr != nil {
			tb.Fatal(retryErr)
		}
		program, err := builder.Seal()
		if err != nil {
			tb.Fatal(err)
		}
		programs[i], leaves[i] = program, leaf
	}
	return programs, leaves
}

func checkAdmittedOrderedBatch(tb testing.TB, programs [4]*valuepkg.Program, ids [4]valuepkg.TypeID) {
	tb.Helper()
	for i, program := range programs {
		var scratch valuepkg.Scratch
		rows := [3]admittedOrderedRow{{ids[i], "2", ""}, {ids[i], "1", "lower bound facet failed"}, {ids[i], "3", ""}}
		switch i {
		case 1:
			rows = [3]admittedOrderedRow{{ids[i], "2", "lower bound facet failed"}, {ids[i], "3", ""}, {ids[i], "8", ""}}
		case 2:
			rows = [3]admittedOrderedRow{{ids[i], "9", ""}, {ids[i], "10", "upper bound facet failed"}, {ids[i], "8", ""}}
		case 3:
			rows = [3]admittedOrderedRow{{ids[i], "9", "upper bound facet failed"}, {ids[i], "3", ""}, {ids[i], "8", ""}}
		}
		for _, row := range rows {
			checkAdmittedOrderedRow(tb, program, row, &scratch)
			if row.message != "" {
				checkAdmittedOrderedRow(tb, program, rows[2], &scratch)
			}
		}
	}
}

func BenchmarkAdmittedOrderedBoundConstruction(b *testing.B) {
	programs, ids := buildAdmittedOrderedBatch(b, true)
	checkAdmittedOrderedBatch(b, programs, ids)
	programs, ids = buildAdmittedOrderedBatch(b, false)
	checkAdmittedOrderedBatch(b, programs, ids)
	b.ReportAllocs()
	for b.Loop() {
		programs, ids = buildAdmittedOrderedBatch(b, false)
	}
	checkAdmittedOrderedBatch(b, programs, ids)
}

func checkAdmittedOrderedTimeContracts(tb testing.TB) {
	tb.Helper()
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	facets := admittedOrderedFacet(valuepkg.FacetMinInclusive, "10:30:00Z", false)
	facets.MaxInclusive = admittedOrderedFacet(valuepkg.FacetMaxInclusive, "10:30:00Z", false).MaxInclusive
	id := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveTime, valuepkg.NoType, facets))
	program, err := builder.Seal()
	if err != nil {
		tb.Fatal(err)
	}
	var scratch valuepkg.Scratch
	checkAdmittedOrderedRow(tb, program, admittedOrderedRow{id, "00:30:00+14:00", ""}, &scratch)
	for _, scenario := range [3]struct {
		base, derived string
		reject        bool
	}{
		{"12:00:00-10:00", "12:00:00-14:00", true},
		{"12:00:00", "12:00:00Z", true},
		{"22:00:00Z", "00:00:00", false},
	} {
		builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
		base := completeAdmittedOrdered(tb, builder, admittedOrderedSpec(valuepkg.PrimitiveTime, valuepkg.NoType, admittedOrderedFacet(valuepkg.FacetMaxInclusive, scenario.base, false)))
		derived, err := builder.Reserve()
		if err != nil {
			tb.Fatal(err)
		}
		err = builder.Complete(derived, admittedOrderedSpec(valuepkg.PrimitiveTime, base, admittedOrderedFacet(valuepkg.FacetMaxInclusive, scenario.derived, false)))
		if scenario.reject {
			checkAdmittedOrderedCompletionError(tb, err, "upper ordered facet cannot loosen base restriction")
			if retryErr := builder.Complete(derived, admittedOrderedSpec(valuepkg.PrimitiveTime, base, valuepkg.FacetSpec{})); retryErr != nil {
				tb.Fatal(retryErr)
			}
		} else if err != nil {
			tb.Fatal(err)
		}
		if _, err := builder.Seal(); err != nil {
			tb.Fatal(err)
		}
	}
}

func TestAdmittedOrderedBoundsBenchmarkPreflight(t *testing.T) {
	for _, fixture := range admittedOrderedFixtures(t) {
		var scratch valuepkg.Scratch
		checkAdmittedOrderedFixture(t, fixture, &scratch)
	}
	for _, widening := range [2]bool{false, true} {
		programs, ids := buildAdmittedOrderedBatch(t, widening)
		checkAdmittedOrderedBatch(t, programs, ids)
	}
	checkAdmittedOrderedTimeContracts(t)
}
