package value

import (
	"strings"
	"testing"
)

func TestDecimalTotalDigitsCountsSignificantFractionDigitsAfterZeroInteger(t *testing.T) {
	for _, test := range []struct {
		name    string
		limit   uint32
		lexical string
		valid   bool
	}{
		{name: "below", limit: 2, lexical: "0", valid: true},
		{name: "at", limit: 2, lexical: "0.05", valid: true},
		{name: "above", limit: 2, lexical: "-0.005", valid: false},
		{name: "trailing zeroes", limit: 2, lexical: "0.0500", valid: true},
		{name: "zero", limit: 1, lexical: "0.00", valid: true},
		{name: "one", limit: 1, lexical: "1.00", valid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, id := decimalTotalDigitsProgram(t, test.limit)
			_, err := program.Validate(id, test.lexical, Resolver{}, 0, 16<<20, nil)
			if (err == nil) != test.valid {
				t.Fatalf("Validate(%q) error = %v, valid = %t", test.lexical, err, test.valid)
			}
		})
	}
}

func TestDecimalParseCountsLeadingFractionZeros(t *testing.T) {
	for _, test := range []struct {
		lexical string
		want    uint32
	}{
		{lexical: "0.05", want: 2},
		{lexical: "0.005", want: 3},
		{lexical: "0.0", want: 1},
	} {
		parsed, err := ParseDecimalValue(test.lexical)
		if err != nil {
			t.Fatalf("ParseDecimalValue(%q): %v", test.lexical, err)
		}
		if parsed.TotalDigits != test.want {
			t.Errorf("ParseDecimalValue(%q).TotalDigits = %d, want %d", test.lexical, parsed.TotalDigits, test.want)
		}
	}
}

func decimalTotalDigitsProgram(t *testing.T, totalDigits uint32) (*Program, TypeID) {
	t.Helper()
	builder := NewBuilder(BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(id, TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{TotalDigits: CardinalityFacet{
			Value:   totalDigits,
			Present: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	return program, id
}

func TestDecimalComparisonValueSpace(t *testing.T) {
	parse := func(t *testing.T, lexical string) DecimalValue {
		t.Helper()
		parsed, err := ParseDecimalValue(lexical)
		if err != nil {
			t.Fatalf("ParseDecimalValue(%q): %v", lexical, err)
		}
		return parsed
	}
	check := func(t *testing.T, left, right DecimalValue, want int) {
		t.Helper()
		if got := CompareDecimalValues(left, right); got != want {
			t.Errorf("CompareDecimalValues(left, right) = %d, want %d", got, want)
		}
		if got := CompareDecimalValues(right, left); got != -want {
			t.Errorf("CompareDecimalValues(right, left) = %d, want %d", got, -want)
		}
	}
	t.Run("zero value and signed zero", func(t *testing.T) {
		check(t, DecimalValue{}, parse(t, "-000.000"), 0)
	})
	prefix := strings.Repeat("7", 1024)
	for _, test := range []struct {
		name        string
		left, right string
		want        int
	}{
		{"absent integer", ".5", "0.5000", 0},
		{"fraction below integer", ".5", "1", -1},
		{"sign and leading zeros", "-00012.0", "+0012", -1},
		{"integer length", "9", "10", -1},
		{"negative integer length", "-9", "-10", 1},
		{"first integer digit", "123", "223", -1},
		{"middle integer digit", "123", "133", -1},
		{"last integer digit", "123", "124", -1},
		{"negative last integer digit", "-123", "-124", 1},
		{"fraction trailing zeros", "123.45", "123.4500", 0},
		{"fraction after equal integer", "123.45", "123.451", -1},
		{"long integer prefix", prefix + "8", prefix + "9", -1},
		{"long negative integer prefix", "-" + prefix + "8", "-" + prefix + "9", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			check(t, parse(t, test.left), parse(t, test.right), test.want)
		})
	}
}

func TestDecimalProjectionIndependence(t *testing.T) {
	zeroValue := DecimalValue{}
	if zeroValue.CanonicalText() != "0.0" || zeroValue.IntegerCanonicalText() != "0" {
		t.Fatal("zero-value canonical projections")
	}
	negative, err := ParseDecimalValue("-00012")
	if err != nil {
		t.Fatal(err)
	}
	negative.Canonical, negative.IntegerCanonical = "999.0", "999"
	negative.TotalDigits, negative.FractionDigits = 0, 0
	if negative.CanonicalText() != "999.0" || negative.IntegerCanonicalText() != "999" {
		t.Fatal("canonical cache overrides were not preserved")
	}
	equal, err := ParseDecimalValue("-12")
	if err != nil {
		t.Fatal(err)
	}
	if !negative.IsNegative() || CompareDecimalValues(negative, equal) != 0 {
		t.Fatal("public projections changed the admitted negative value")
	}
	for _, lexical := range []string{"-0", "-.000", "-000."} {
		t.Run(lexical, func(t *testing.T) {
			zero, parseErr := ParseDecimalValue(lexical)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if zero.CanonicalText() != "0.0" || zero.IsNegative() || CompareDecimalValues(zero, DecimalValue{}) != 0 {
				t.Fatal("signed zero differs from the zero value")
			}
			zero.Canonical, zero.IntegerCanonical = "999.0", "999"
			zero.TotalDigits, zero.FractionDigits = 9, 9
			if zero.IsNegative() || CompareDecimalValues(zero, DecimalValue{}) != 0 {
				t.Fatal("public projections changed the admitted zero value")
			}
		})
	}
}

func TestDecimalFactoryCanonicalCaches(t *testing.T) {
	for _, test := range []struct {
		lexical, canonical string
	}{
		{"+00012", "12.0"},
		{"-000.000", "0.0"},
		{"-00012.3400", "-12.34"},
	} {
		t.Run(test.lexical, func(t *testing.T) {
			plain, err := ParseDecimalValue(test.lexical)
			if err != nil {
				t.Fatal(err)
			}
			if plain.Canonical != "" || plain.IntegerCanonical != "" {
				t.Fatalf("value-only caches = %q/%q, want empty", plain.Canonical, plain.IntegerCanonical)
			}
			if got := plain.CanonicalText(); got != test.canonical {
				t.Fatalf("value-only canonical = %q, want %q", got, test.canonical)
			}
			cached, err := ParseDecimalCanonical(test.lexical)
			if err != nil {
				t.Fatal(err)
			}
			if cached.Canonical != test.canonical {
				t.Fatalf("canonical cache = %q, want %q", cached.Canonical, test.canonical)
			}
			if test.lexical == "+00012" && cached.IntegerCanonical != "12" {
				t.Fatalf("integer canonical cache = %q, want 12", cached.IntegerCanonical)
			}
		})
	}
}

func TestDecimalNormalizedFractionOrdering(t *testing.T) {
	prefix := strings.Repeat("7", 1024)
	for _, test := range []struct {
		name, left, right string
		want              int
	}{
		{"negative prefix", "-0.12", "-0.120001", 1},
		{"long late difference", "0." + prefix + "8", "0." + prefix + "9", -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			left, err := ParseDecimalValue(test.left)
			if err != nil {
				t.Fatal(err)
			}
			right, err := ParseDecimalValue(test.right)
			if err != nil {
				t.Fatal(err)
			}
			if got := CompareDecimalValues(left, right); got != test.want {
				t.Fatalf("CompareDecimalValues(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
			}
		})
	}
}
