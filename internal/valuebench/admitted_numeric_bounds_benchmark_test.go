package valuebench

import (
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

type admittedNumericFixture struct {
	name, min, max, input string
	builtin               valuepkg.BuiltinKind
	opponents             []struct{ input, message string }
}

func admittedNumericFixtures() [3]admittedNumericFixture {
	return [3]admittedNumericFixture{
		{name: "NegativePaddedInteger", min: "-0002", max: "-0001", input: "-1", builtin: valuepkg.BuiltinInteger,
			opponents: []struct{ input, message string }{{"-3", "minInclusive facet failed"}, {"0", "maxInclusive facet failed"}}},
		{name: "SignedZeroInteger", min: "-0000", max: "+0001", input: "-0000", builtin: valuepkg.BuiltinInteger,
			opponents: []struct{ input, message string }{{"+0001", ""}, {"-1", "minInclusive facet failed"}, {"2", "maxInclusive facet failed"}, {"1.0", "invalid integer"}}},
		{name: "LongPaddedDecimal", min: "0", max: "+" + strings.Repeat("0", 1024) + "1.5000", input: "1.5",
			opponents: []struct{ input, message string }{{"1.5001", "maxInclusive facet failed"}}},
	}
}

func admittedNumericSpec(f admittedNumericFixture) valuepkg.TypeSpec {
	return valuepkg.TypeSpec{
		Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveDecimal, Builtin: f.builtin,
		Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true, Base: valuepkg.NoType, ListItem: valuepkg.NoType,
		Facets: valuepkg.FacetSpec{
			MinInclusive: valuepkg.BoundFacet{LiteralSpec: valuepkg.LiteralSpec{Lexical: f.min, Type: valuepkg.NoType}, Present: true},
			MaxInclusive: valuepkg.BoundFacet{LiteralSpec: valuepkg.LiteralSpec{Lexical: f.max, Type: valuepkg.NoType}, Present: true},
		},
	}
}

func BenchmarkAdmittedNumericBounds(b *testing.B) {
	for _, fixture := range admittedNumericFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				b.Fatal(err)
			}
			if completeErr := builder.Complete(id, admittedNumericSpec(fixture)); completeErr != nil {
				b.Fatal(completeErr)
			}
			program, err := builder.Seal()
			if err != nil {
				b.Fatal(err)
			}
			var scratch valuepkg.Scratch
			checkAdmittedNumericFixture(b, program, id, fixture, &scratch)
			raw := []byte(fixture.input)
			var last valuepkg.Value
			b.ReportAllocs()
			for b.Loop() {
				last, err = program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, &scratch)
				if err != nil {
					b.Fatal(err)
				}
			}
			checkRawPlanResult(b, last, id)
			checkAdmittedNumericFixture(b, program, id, fixture, &scratch)
		})
	}
}

func BenchmarkColdNumericBoundConstruction(b *testing.B) {
	fixtures := admittedNumericFixtures()
	selected := [2]admittedNumericFixture{fixtures[0], fixtures[2]}
	specs := [2]valuepkg.TypeSpec{admittedNumericSpec(selected[0]), admittedNumericSpec(selected[1])}
	program, ids, err := buildAdmittedNumericPair(specs)
	if err != nil {
		b.Fatal(err)
	}
	check := func() {
		var scratch valuepkg.Scratch
		for i, fixture := range selected {
			checkAdmittedNumericFixture(b, program, ids[i], fixture, &scratch)
		}
	}
	check()
	b.ReportAllocs()
	for b.Loop() {
		program, ids, err = buildAdmittedNumericPair(specs)
		if err != nil {
			b.Fatal(err)
		}
	}
	check()
}

func buildAdmittedNumericPair(specs [2]valuepkg.TypeSpec) (*valuepkg.Program, [2]valuepkg.TypeID, error) {
	var ids [2]valuepkg.TypeID
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	for i, spec := range specs {
		id, err := builder.Reserve()
		if err != nil {
			return nil, ids, err
		}
		ids[i] = id
		if completeErr := builder.Complete(id, spec); completeErr != nil {
			return nil, ids, completeErr
		}
	}
	program, err := builder.Seal()
	return program, ids, err
}

func checkAdmittedNumericFixture(tb testing.TB, program *valuepkg.Program, id valuepkg.TypeID, f admittedNumericFixture, scratch *valuepkg.Scratch) {
	tb.Helper()
	for _, opponent := range f.opponents {
		got, err := program.ValidateBytes(id, []byte(opponent.input), valuepkg.Resolver{}, 0, 16<<20, scratch)
		if opponent.message != "" {
			if err == nil || err.Error() != opponent.message {
				tb.Fatalf("ValidateBytes(%q) = %v, want %q", opponent.input, err, opponent.message)
			}
		} else {
			if err != nil {
				tb.Fatal(err)
			}
			checkRawPlanResult(tb, got, id)
		}
	}
	got, err := program.ValidateBytes(id, []byte(f.input), valuepkg.Resolver{}, 0, 16<<20, scratch)
	if err != nil {
		tb.Fatal(err)
	}
	checkRawPlanResult(tb, got, id)
}
