package runtime

import "testing"

func TestParseGValueCanonicalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		canonical string
		kind      PrimitiveKind
	}{
		{name: "gYearMonth", kind: PrimitiveGYearMonth, input: "2026-05Z", canonical: "2026-05Z"},
		{name: "gYear", kind: PrimitiveGYear, input: "-0001+00:00", canonical: "-0001Z"},
		{name: "gMonthDay", kind: PrimitiveGMonthDay, input: "--02-29+00:00", canonical: "--02-29Z"},
		{name: "gDay", kind: PrimitiveGDay, input: "---31-14:00", canonical: "---31-14:00"},
		{name: "gMonth", kind: PrimitiveGMonth, input: "--12", canonical: "--12"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseGValue(tt.kind, tt.input)
			if err != nil {
				t.Fatalf("ParseGValue() error = %v", err)
			}
			if got.CanonicalText() != tt.canonical {
				t.Fatalf("GValue.CanonicalText() = %q, want %q", got.CanonicalText(), tt.canonical)
			}
		})
	}
}

func TestParseGValueRejectsInvalidPrimitive(t *testing.T) {
	t.Parallel()

	if _, err := ParseGValue(PrimitiveString, "2000"); err == nil || err.Error() != "invalid g value primitive" {
		t.Fatalf("ParseGValue() error = %v, want invalid g value primitive", err)
	}
}

func TestGValueEqualityAndPartialOrder(t *testing.T) {
	t.Parallel()

	a, err := ParseGValue(PrimitiveGMonth, "--05Z")
	if err != nil {
		t.Fatalf("ParseGValue(a) error = %v", err)
	}
	b, err := ParseGValue(PrimitiveGMonth, "--05+00:00")
	if err != nil {
		t.Fatalf("ParseGValue(b) error = %v", err)
	}
	if !EqualGValues(a, b) {
		t.Fatal("EqualGValues() = false, want true")
	}

	absent, err := ParseGValue(PrimitiveGDay, "---10")
	if err != nil {
		t.Fatalf("ParseGValue(absent) error = %v", err)
	}
	z, err := ParseGValue(PrimitiveGDay, "---10Z")
	if err != nil {
		t.Fatalf("ParseGValue(z) error = %v", err)
	}
	if got := CompareGValues(absent, z); got != OrderedFacetIncomparable {
		t.Fatalf("CompareGValues(absent, z) = %v, want incomparable", got)
	}
}
