package value

import (
	"math"
	"testing"
)

func TestFormatFloatCanonical(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		bits  int
		want  string
	}{
		{name: "float one", value: 1, bits: 32, want: "1.0E0"},
		{name: "float shortest", value: 1.5, bits: 32, want: "1.5E0"},
		{name: "float exponent", value: 100, bits: 32, want: "1.0E2"},
		{name: "float fraction", value: 0.1, bits: 32, want: "1.0E-1"},
		{name: "float minimum", value: math.SmallestNonzeroFloat32, bits: 32, want: "1.0E-45"},
		{name: "float maximum", value: math.MaxFloat32, bits: 32, want: "3.4028235E38"},
		{name: "double shortest", value: 1.2345678901234567, bits: 64, want: "1.2345678901234567E0"},
		{name: "double minimum", value: math.SmallestNonzeroFloat64, bits: 64, want: "5.0E-324"},
		{name: "double maximum", value: math.MaxFloat64, bits: 64, want: "1.7976931348623157E308"},
		{name: "negative", value: -0.25, bits: 64, want: "-2.5E-1"},
		{name: "positive zero", value: 0, bits: 64, want: "0.0E0"},
		{name: "negative zero", value: math.Copysign(0, -1), bits: 64, want: "0.0E0"},
		{name: "positive infinity", value: math.Inf(1), bits: 64, want: "INF"},
		{name: "negative infinity", value: math.Inf(-1), bits: 64, want: "-INF"},
		{name: "not a number", value: math.NaN(), bits: 64, want: "NaN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatFloatCanonical(tc.value, tc.bits); got != tc.want {
				t.Fatalf("formatFloatCanonical(%v, %d) = %q, want %q", tc.value, tc.bits, got, tc.want)
			}
		})
	}
}

func TestParseFloatCanonicalProjection(t *testing.T) {
	cases := []struct {
		kind    PrimitiveKind
		lexical string
		want    string
	}{
		{kind: PrimitiveFloat, lexical: "1e2", want: "1.0E2"},
		{kind: PrimitiveDouble, lexical: "-0", want: "0.0E0"},
		{kind: PrimitiveDouble, lexical: "INF", want: "INF"},
		{kind: PrimitiveDouble, lexical: "-INF", want: "-INF"},
		{kind: PrimitiveDouble, lexical: "NaN", want: "NaN"},
	}
	for _, tc := range cases {
		t.Run(tc.lexical, func(t *testing.T) {
			got, err := ParseFloatValue(tc.kind, tc.lexical, PrimitiveNeedCanonical)
			if err != nil {
				t.Fatalf("ParseFloatValue(%q): %v", tc.lexical, err)
			}
			if got.Canonical != tc.want {
				t.Fatalf("canonical(%q) = %q, want %q", tc.lexical, got.Canonical, tc.want)
			}
		})
	}
}
