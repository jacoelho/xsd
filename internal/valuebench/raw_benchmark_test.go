package valuebench

import (
	"errors"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func BenchmarkPublishedRawStringEnumeration(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(id, valuepkg.TypeSpec{
		Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString,
		Whitespace: valuepkg.WhitespacePreserve, WhitespacePresent: true,
		Base: valuepkg.NoType, ListItem: valuepkg.NoType,
		Facets: valuepkg.FacetSpec{Enumeration: []valuepkg.LiteralSpec{{Lexical: "ready"}, {Lexical: "waiting"}}},
	})
	if err != nil {
		b.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	raw := []byte("ready")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublishedRawNMTokensList(b *testing.B) {
	program, ok := mustBuiltinProgram(b, "NMTOKENS")
	if !ok {
		b.Fatal("missing NMTOKENS builtin")
	}
	id, _ := valuepkg.BuiltinTypeID("NMTOKENS")
	raw := []byte("one two three")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublishedRawIntegerBounds(b *testing.B) {
	program, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Seal()
	if err != nil {
		b.Fatal(err)
	}
	id, _ := valuepkg.BuiltinTypeID("unsignedLong")
	raw := []byte("18446744073709551615")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func mustBuiltinProgram(tb testing.TB, name string) (*valuepkg.Program, bool) {
	tb.Helper()
	program, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Seal()
	if err != nil {
		tb.Fatal(err)
	}
	_, ok := valuepkg.BuiltinTypeID(name)
	return program, ok
}

type admittedNumericFixture struct {
	name, min, max, input string
	builtin               valuepkg.BuiltinKind
	opponents             []struct{ input, message string }
}

func admittedNumericFixtures() [3]admittedNumericFixture {
	return [3]admittedNumericFixture{
		{name: "NegativePaddedInteger", min: "-0002", max: "-0001", input: "-1", builtin: valuepkg.BuiltinInteger,
			opponents: []struct{ input, message string }{{"-3", `value "-3" does not satisfy minInclusive "-2.0"`}, {"0", `value "0" does not satisfy maxInclusive "-1.0"`}}},
		{name: "SignedZeroInteger", min: "-0000", max: "+0001", input: "-0000", builtin: valuepkg.BuiltinInteger,
			opponents: []struct{ input, message string }{{"+0001", ""}, {"-1", `value "-1" does not satisfy minInclusive "0.0"`}, {"2", `value "2" does not satisfy maxInclusive "1.0"`}, {"1.0", `value "1.0" is not valid for xs:integer: invalid integer`}}},
		{name: "LongPaddedDecimal", min: "0", max: "+" + strings.Repeat("0", 1024) + "1.5000", input: "1.5",
			opponents: []struct{ input, message string }{{"1.5001", `value "1.5001" does not satisfy maxInclusive "1.5"`}}},
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

func BenchmarkCanonicalMagnitude(b *testing.B) {
	parse := func(tb testing.TB, lexical string) valuepkg.DecimalValue {
		tb.Helper()
		got, err := valuepkg.ParseDecimalValue(lexical)
		if err != nil {
			tb.Fatal(err)
		}
		return got
	}
	prefix := strings.Repeat("7", 1024)
	for _, test := range []struct {
		name, left, right string
		want              int
	}{
		{"CompareShortNegative", "-1", "-2", 1},
		{"CompareSignedZero", "-000.0000", "0", 0},
		{"CompareLongIntegerPrefix", prefix + "8", prefix + "9", -1},
	} {
		b.Run(test.name, func(b *testing.B) {
			left, right := parse(b, test.left), parse(b, test.right)
			if test.name == "CompareSignedZero" {
				right = valuepkg.DecimalValue{}
			}
			if test.name == "CompareLongIntegerPrefix" && valuepkg.CompareDecimalValues(parse(b, "-"+prefix+"8"), parse(b, "-"+prefix+"9")) != 1 {
				b.Fatal("negative long-prefix order")
			}
			for b.Loop() {
				if valuepkg.CompareDecimalValues(left, right) != test.want {
					b.Fatal("decimal order")
				}
			}
		})
	}
	for _, test := range []struct {
		name, lexical, canonical, integer string
		negative                          bool
	}{
		{"ParseSignedZero", "-000.0000", "0.0", "", false},
		{"ParsePaddedNegativeInteger", "-" + strings.Repeat("0", 1024) + "1", "-1.0", "-1", true},
	} {
		b.Run(test.name, func(b *testing.B) {
			var got valuepkg.DecimalValue
			var err error
			for b.Loop() {
				got, err = valuepkg.ParseDecimalValue(test.lexical)
				if err != nil {
					b.Fatal(err)
				}
			}
			if got.IsNegative() != test.negative || got.CanonicalText() != test.canonical || test.integer != "" && got.IntegerCanonicalText() != test.integer {
				b.Fatal("parsed projections")
			}
		})
	}
	b.Run("CanonicalNegativeFraction", func(b *testing.B) {
		got := parse(b, "-.5000")
		for b.Loop() {
			if got.CanonicalText() != "-0.5" {
				b.Fatal("fraction canonicalization")
			}
		}
	})
	b.Run("PresentZeroRaw", func(b *testing.B) {
		builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
		id, err := builder.Reserve()
		if err != nil {
			b.Fatal(err)
		}
		bound := valuepkg.BoundFacet{Lexical: "0", Type: valuepkg.NoType, Present: true}
		err = builder.Complete(id, valuepkg.TypeSpec{Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveDecimal,
			Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true, Base: valuepkg.NoType, ListItem: valuepkg.NoType,
			Facets: valuepkg.FacetSpec{MinInclusive: bound, MaxInclusive: bound}})
		if err != nil {
			b.Fatal(err)
		}
		program, err := builder.Seal()
		if err != nil {
			b.Fatal(err)
		}
		var scratch valuepkg.Scratch
		for _, test := range []struct{ input, message string }{{"-.1", `value "-.1" does not satisfy minInclusive "0.0"`}, {".1", `value ".1" does not satisfy maxInclusive "0.0"`}, {"0", ""}} {
			_, checkErr := program.ValidateBytes(id, []byte(test.input), valuepkg.Resolver{}, 0, 1<<20, &scratch)
			if test.message == "" {
				if checkErr != nil {
					b.Fatal(checkErr)
				}
			} else if checkErr == nil || checkErr.Error() != test.message {
				b.Fatalf("%s: %v", test.input, checkErr)
			}
		}
		raw := []byte("-000.0000")
		got, err := program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 1<<20, &scratch)
		if err != nil {
			b.Fatal(err)
		}
		checkRawPlanResult(b, got, id)
		for b.Loop() {
			got, err = program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 1<<20, &scratch)
			if err != nil {
				b.Fatal(err)
			}
		}
		checkRawPlanResult(b, got, id)
	})
}

func BenchmarkPublishedRawDecimalBounds(b *testing.B) {
	for _, test := range []struct {
		name, min, max, input, message string
		fraction                       bool
	}{
		{name: "NoBounds", input: "42.50"},
		{name: "MinOnly", min: ".5", input: ".5"},
		{name: "MaxOnly", max: "+000.5000", input: ".5"},
		{name: "Both", min: "0", max: "999999", input: "42.50"},
		{name: "NegativeFallback", min: "-2", max: "-1", input: "-1.5"},
		{name: "WhitespaceFallback", min: "0", max: "999999", input: " \t42.50\n"},
		{name: "MaxReject", min: "0", max: "999999", input: "1000000", message: `value "1000000" does not satisfy maxInclusive "999999.0"`},
		{name: "UnsupportedFractionDigits", min: "0", max: "10", fraction: true, input: "1.25", message: `value "1.25" has 2 fractional digits; fractionDigits permits at most 1`},
	} {
		b.Run(test.name, func(b *testing.B) {
			builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				b.Fatal(err)
			}
			var facets valuepkg.FacetSpec
			if test.min != "" {
				facets.MinInclusive = valuepkg.BoundFacet{Lexical: test.min, Type: valuepkg.NoType, Present: true}
			}
			if test.max != "" {
				facets.MaxInclusive = valuepkg.BoundFacet{Lexical: test.max, Type: valuepkg.NoType, Present: true}
			}
			if test.fraction {
				facets.FractionDigits = valuepkg.CardinalityFacet{Value: 1, Present: true}
			}
			if completeErr := builder.Complete(id, valuepkg.TypeSpec{
				Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveDecimal,
				Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
				Base: valuepkg.NoType, ListItem: valuepkg.NoType, Facets: facets,
			}); completeErr != nil {
				b.Fatal(completeErr)
			}
			program, err := builder.Seal()
			if err != nil {
				b.Fatal(err)
			}
			raw := []byte(test.input)
			var scratch valuepkg.Scratch
			check := func(input, message string, facet bool) valuepkg.Value {
				got, validationErr := program.ValidateBytes(id, []byte(input), valuepkg.Resolver{}, 0, 16<<20, &scratch)
				if message != "" {
					if validationErr == nil || validationErr.Error() != message || facet && !errors.Is(validationErr, valuepkg.ErrFacet) {
						b.Fatalf("ValidateBytes(%q) = %v, want %q", input, validationErr, message)
					}
					return got
				}
				if validationErr != nil {
					b.Fatal(validationErr)
				}
				checkRawPlanResult(b, got, id)
				return got
			}
			preflight := func() {
				check(test.input, test.message, test.fraction)
				if test.name == "NegativeFallback" {
					check("0", `value "0" does not satisfy maxInclusive "-1.0"`, true)
					check("-1.5", "", false)
				}
				if test.fraction {
					check("1.25", `value "1.25" has 2 fractional digits; fractionDigits permits at most 1`, true)
					check("1.20", "", false)
				}
			}
			preflight()
			var last valuepkg.Value
			b.ReportAllocs()
			for b.Loop() {
				last, err = program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, &scratch)
				if test.message == "" {
					if err != nil {
						b.Fatal(err)
					}
				} else if err == nil || err.Error() != test.message || test.fraction && !errors.Is(err, valuepkg.ErrFacet) {
					b.Fatalf("ValidateBytes(%q) = %v, want %q", test.input, err, test.message)
				}
			}
			if test.message == "" {
				checkRawPlanResult(b, last, id)
			}
			preflight()
		})
	}
}

func checkRawPlanResult(tb testing.TB, got valuepkg.Value, id valuepkg.TypeID) {
	tb.Helper()
	if got.Type() != id || got.SelectedType() != valuepkg.NoType || got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
		tb.Fatalf("unprojected decimal result = %#v", got)
	}
}
