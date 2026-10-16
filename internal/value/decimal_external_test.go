package value_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestDecimalComparisonValueSpace(t *testing.T) {
	parse := func(t *testing.T, lexical string) value.DecimalValue {
		t.Helper()
		parsed, err := value.ParseDecimalValue(lexical)
		if err != nil {
			t.Fatalf("ParseDecimalValue(%q): %v", lexical, err)
		}
		return parsed
	}
	check := func(t *testing.T, left, right value.DecimalValue, want int) {
		t.Helper()
		if got := value.CompareDecimalValues(left, right); got != want {
			t.Errorf("CompareDecimalValues(left, right) = %d, want %d", got, want)
		}
		if got := value.CompareDecimalValues(right, left); got != -want {
			t.Errorf("CompareDecimalValues(right, left) = %d, want %d", got, -want)
		}
	}
	t.Run("zero value and signed zero", func(t *testing.T) {
		check(t, value.DecimalValue{}, parse(t, "-000.000"), 0)
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
	zeroValue := value.DecimalValue{}
	if zeroValue.CanonicalText() != "0.0" || zeroValue.IntegerCanonicalText() != "0" {
		t.Fatal("zero-value canonical projections")
	}
	negative, err := value.ParseDecimalValue("-00012")
	if err != nil {
		t.Fatal(err)
	}
	negative.Canonical, negative.IntegerCanonical = "999.0", "999"
	negative.TotalDigits, negative.FractionDigits = 0, 0
	if negative.CanonicalText() != "999.0" || negative.IntegerCanonicalText() != "999" {
		t.Fatal("canonical cache overrides were not preserved")
	}
	equal, err := value.ParseDecimalValue("-12")
	if err != nil {
		t.Fatal(err)
	}
	if !negative.IsNegative() || value.CompareDecimalValues(negative, equal) != 0 {
		t.Fatal("public projections changed the admitted negative value")
	}
	for _, lexical := range []string{"-0", "-.000", "-000."} {
		t.Run(lexical, func(t *testing.T) {
			zero, parseErr := value.ParseDecimalValue(lexical)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if zero.CanonicalText() != "0.0" || zero.IsNegative() || value.CompareDecimalValues(zero, value.DecimalValue{}) != 0 {
				t.Fatal("signed zero differs from the zero value")
			}
			zero.Canonical, zero.IntegerCanonical = "999.0", "999"
			zero.TotalDigits, zero.FractionDigits = 9, 9
			if zero.IsNegative() || value.CompareDecimalValues(zero, value.DecimalValue{}) != 0 {
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
			plain, err := value.ParseDecimalValue(test.lexical)
			if err != nil {
				t.Fatal(err)
			}
			if plain.Canonical != "" || plain.IntegerCanonical != "" {
				t.Fatalf("value-only caches = %q/%q, want empty", plain.Canonical, plain.IntegerCanonical)
			}
			if got := plain.CanonicalText(); got != test.canonical {
				t.Fatalf("value-only canonical = %q, want %q", got, test.canonical)
			}
			cached, err := value.ParseDecimalCanonical(test.lexical)
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

func TestDecimalFactoriesRejectExponent(t *testing.T) {
	for name, parse := range map[string]func(string) (value.DecimalValue, error){
		"value":     value.ParseDecimalValue,
		"canonical": value.ParseDecimalCanonical,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parse("1e2"); err == nil || err.Error() != "invalid decimal" {
				t.Fatalf("exponent error = %v, want invalid decimal", err)
			}
		})
	}
}

func TestDecimalAndIntegerProjectionRequests(t *testing.T) {
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		builtin, lexical, canonical, identity string
	}{
		{"integer", "-00012", "-12", "\x02\x1e-12.0"},
		{"decimal", "-00012", "-12.0", "\x02\x1e-12.0"},
		{"integer", "-000", "0", "\x02\x1e0.0"},
		{"decimal", "-000.000", "0.0", "\x02\x1e0.0"},
		{"decimal", "-00012.3400", "-12.34", "\x02\x1e-12.34"},
	} {
		t.Run(test.builtin+"/"+test.lexical, func(t *testing.T) {
			id, ok := value.BuiltinTypeID(test.builtin)
			if !ok {
				t.Fatalf("missing builtin %s", test.builtin)
			}
			for _, needs := range []value.Needs{0, value.NeedCanonical, value.NeedIdentity, value.NeedCanonical | value.NeedIdentity} {
				for _, bytes := range []bool{false, true} {
					var got value.Value
					if bytes {
						raw := []byte(test.lexical)
						got, err = program.ValidateBytes(id, raw, value.Resolver{}, needs, 16<<20, nil)
						clear(raw)
					} else {
						got, err = program.Validate(id, test.lexical, value.Resolver{}, needs, 16<<20, nil)
					}
					if err != nil {
						t.Fatalf("needs %d, bytes %t: %v", needs, bytes, err)
					}
					if got.Type() != id || got.SelectedType() != value.NoType {
						t.Fatalf("needs %d, bytes %t: type/selection=%d/%d, want %d/NoType", needs, bytes, got.Type(), got.SelectedType(), id)
					}
					assertRequestedCanonicalProjection(t, got, needs, test.canonical, test.identity)
					if got.IDs() != "" || got.IDRefs() != "" {
						t.Fatalf("needs %d, bytes %t: unsolicited document identity %q/%q", needs, bytes, got.IDs(), got.IDRefs())
					}
				}
			}
		})
	}
}
