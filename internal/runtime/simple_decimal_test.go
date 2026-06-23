package runtime

import (
	"errors"
	"testing"
)

func TestParseDecimalCanonical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in             string
		canonical      string
		integer        string
		integerLexical bool
		totalDigits    uint32
		fractionDigits uint32
	}{
		{in: "7", canonical: "7.0", integer: "7", integerLexical: true, totalDigits: 1, fractionDigits: 0},
		{in: "+000.0100", canonical: "0.01", integer: "0", integerLexical: false, totalDigits: 1, fractionDigits: 2},
		{in: "-000", canonical: "0.0", integer: "0", integerLexical: true, totalDigits: 1, fractionDigits: 0},
		{in: ".50", canonical: "0.5", integer: "0", integerLexical: false, totalDigits: 1, fractionDigits: 1},
		{in: "-.50", canonical: "-0.5", integer: "0", integerLexical: false, totalDigits: 1, fractionDigits: 1},
		{in: "-000123.4500", canonical: "-123.45", integer: "-123", integerLexical: false, totalDigits: 5, fractionDigits: 2},
		{in: "5.", canonical: "5.0", integer: "5", integerLexical: false, totalDigits: 1, fractionDigits: 0},
		{in: "1000.00", canonical: "1000.0", integer: "1000", integerLexical: false, totalDigits: 4, fractionDigits: 0},
		{in: "0.0010", canonical: "0.001", integer: "0", integerLexical: false, totalDigits: 1, fractionDigits: 3},
		{in: "+000000000000000000000001234567890.0000000000001000", canonical: "1234567890.0000000000001", integer: "1234567890", integerLexical: false, totalDigits: 23, fractionDigits: 13},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, err := ParseDecimalCanonical(tt.in)
			if err != nil {
				t.Fatalf("ParseDecimalCanonical() error = %v", err)
			}
			if got.Canonical != tt.canonical || got.IntegerCanonical != tt.integer || got.IntegerLexical != tt.integerLexical || got.TotalDigits != tt.totalDigits || got.FractionDigits != tt.fractionDigits {
				t.Fatalf("ParseDecimalCanonical() = %+v, want canonical=%q integer=%q integerLexical=%v total=%d fraction=%d", got, tt.canonical, tt.integer, tt.integerLexical, tt.totalDigits, tt.fractionDigits)
			}
		})
	}
}

func TestParseDecimalRejectsInvalidLexicalValues(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "+", "-", ".", "+.", "-.", "1.2.3", "12a", "1e2"} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseDecimalCanonical(in); err == nil {
				t.Fatal("ParseDecimalCanonical() error = nil")
			}
		})
	}
}

func TestCompareDecimalValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "integer and decimal equal", a: "5", b: "5.0"},
		{name: "negative zero equals zero", a: "-0", b: "0"},
		{name: "negative fraction below zero", a: "-.5", b: "0", want: -1},
		{name: "fractional padding equal", a: "1.20", b: "1.2"},
		{name: "integer digit length greater", a: "10", b: "9", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a, err := ParseDecimalValue(tt.a)
			if err != nil {
				t.Fatalf("ParseDecimalValue(%q) error = %v", tt.a, err)
			}
			b, err := ParseDecimalValue(tt.b)
			if err != nil {
				t.Fatalf("ParseDecimalValue(%q) error = %v", tt.b, err)
			}
			got := CompareDecimalValues(a, b)
			if tt.want < 0 && got >= 0 || tt.want == 0 && got != 0 || tt.want > 0 && got <= 0 {
				t.Fatalf("CompareDecimalValues(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestValidateDecimalFacets(t *testing.T) {
	t.Parallel()

	decimal := func(tb testing.TB, s string) DecimalValue {
		tb.Helper()
		v, err := ParseDecimalValue(s)
		if err != nil {
			tb.Fatalf("ParseDecimalValue(%q) error = %v", s, err)
		}
		return v
	}
	cardinality := func(v uint32) FacetCardinalityValue {
		return FacetCardinalityValue{Value: v, Present: true}
	}
	bound := func(s string) DecimalFacetValue {
		return DecimalFacetValue{Value: decimal(t, s), Present: true}
	}
	tests := []struct {
		name     string
		facets   DecimalFacetValues
		value    string
		wantErr  string
		wantMeta bool
	}{
		{name: "total digits", facets: DecimalFacetValues{TotalDigits: cardinality(1), Facets: FacetTotalDigits}, value: "12", wantErr: "totalDigits facet failed"},
		{name: "fraction digits", facets: DecimalFacetValues{FractionDigits: cardinality(1), Facets: FacetFractionDigits}, value: "1.23", wantErr: "fractionDigits facet failed"},
		{name: "min inclusive", facets: DecimalFacetValues{MinInclusive: bound("1"), Facets: FacetMinInclusive}, value: "0.9", wantErr: "minInclusive facet failed"},
		{name: "max inclusive", facets: DecimalFacetValues{MaxInclusive: bound("10"), Facets: FacetMaxInclusive}, value: "10.1", wantErr: "maxInclusive facet failed"},
		{name: "min exclusive", facets: DecimalFacetValues{MinExclusive: bound("1"), Facets: FacetMinExclusive}, value: "1", wantErr: "minExclusive facet failed"},
		{name: "max exclusive", facets: DecimalFacetValues{MaxExclusive: bound("10"), Facets: FacetMaxExclusive}, value: "10", wantErr: "maxExclusive facet failed"},
		{name: "negative zero satisfies non-negative min", facets: DecimalFacetValues{MinInclusive: bound("0"), Facets: FacetMinInclusive}, value: "-0.0"},
		{name: "missing bound metadata", facets: DecimalFacetValues{Facets: FacetMinInclusive}, value: "1", wantMeta: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDecimalFacets(tt.facets, decimal(t, tt.value))
			if tt.wantMeta {
				if !errors.Is(err, ErrSimpleValueMetadata) {
					t.Fatalf("ValidateDecimalFacets() error = %v, want metadata sentinel", err)
				}
				return
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateDecimalFacets() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateDecimalFacets() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
