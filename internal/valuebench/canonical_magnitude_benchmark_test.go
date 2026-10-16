package valuebench

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func BenchmarkCanonicalMagnitude(b *testing.B) {
	parse := func(tb testing.TB, lexical string) value.DecimalValue {
		tb.Helper()
		got, err := value.ParseDecimalValue(lexical)
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
				right = value.DecimalValue{}
			}
			if test.name == "CompareLongIntegerPrefix" && value.CompareDecimalValues(parse(b, "-"+prefix+"8"), parse(b, "-"+prefix+"9")) != 1 {
				b.Fatal("negative long-prefix order")
			}
			for b.Loop() {
				if value.CompareDecimalValues(left, right) != test.want {
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
			var got value.DecimalValue
			var err error
			for b.Loop() {
				got, err = value.ParseDecimalValue(test.lexical)
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
		builder := value.NewBuilder(value.BuilderOptions{})
		id, err := builder.Reserve()
		if err != nil {
			b.Fatal(err)
		}
		bound := value.BoundFacet{Lexical: "0", Type: value.NoType, Present: true}
		err = builder.Complete(id, value.TypeSpec{Variety: value.Atomic, Primitive: value.PrimitiveDecimal,
			Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.NoType,
			Facets: value.FacetSpec{MinInclusive: bound, MaxInclusive: bound}})
		if err != nil {
			b.Fatal(err)
		}
		program, err := builder.Seal()
		if err != nil {
			b.Fatal(err)
		}
		var scratch value.Scratch
		for _, test := range []struct{ input, message string }{{"-.1", "minInclusive facet failed"}, {".1", "maxInclusive facet failed"}, {"0", ""}} {
			_, checkErr := program.ValidateBytes(id, []byte(test.input), value.Resolver{}, 0, 1<<20, &scratch)
			if test.message == "" {
				if checkErr != nil {
					b.Fatal(checkErr)
				}
			} else if checkErr == nil || checkErr.Error() != test.message {
				b.Fatalf("%s: %v", test.input, checkErr)
			}
		}
		raw := []byte("-000.0000")
		got, err := program.ValidateBytes(id, raw, value.Resolver{}, 0, 1<<20, &scratch)
		if err != nil {
			b.Fatal(err)
		}
		checkRawPlanResult(b, got, id)
		for b.Loop() {
			got, err = program.ValidateBytes(id, raw, value.Resolver{}, 0, 1<<20, &scratch)
			if err != nil {
				b.Fatal(err)
			}
		}
		checkRawPlanResult(b, got, id)
	})
}
