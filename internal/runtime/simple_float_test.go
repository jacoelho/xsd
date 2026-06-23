package runtime

import (
	"errors"
	"math"
	"testing"
)

func TestValidateFloatLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "zero", input: "0"},
		{name: "signed decimal", input: "-1.25"},
		{name: "leading dot", input: ".5"},
		{name: "trailing dot", input: "5."},
		{name: "exponent", input: "+1.2e-3"},
		{name: "positive infinity", input: "INF"},
		{name: "negative infinity", input: "-INF"},
		{name: "nan", input: "NaN"},
		{name: "range overflow remains valid", input: "1E9999"},
		{name: "rejects signed infinity", input: "+INF", wantErr: "invalid float"},
		{name: "rejects lower nan", input: "nan", wantErr: "invalid float"},
		{name: "rejects missing exponent digits", input: "1e", wantErr: "invalid float"},
		{name: "rejects dot only", input: ".", wantErr: "invalid float"},
		{name: "rejects whitespace", input: "1 2", wantErr: "invalid float"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bytesErr := ValidateFloatLexical([]byte(tt.input), 32)
			if got := errorMessage(bytesErr); got != tt.wantErr {
				t.Fatalf("ValidateFloatLexical() error = %q, want %q", got, tt.wantErr)
			}
			stringErr := ValidateFloatLexical(tt.input, 64)
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("ValidateFloatLexical string error for %q = %v, bytes error = %v", tt.input, stringErr, bytesErr)
			}
		})
	}
}

func TestParseFloatValueCanonicalAndValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		kind      PrimitiveKind
		input     string
		canonical string
		want      func(float64) bool
	}{
		{name: "double zero", kind: PrimitiveDouble, input: "-0", canonical: "0", want: func(v float64) bool { return v == 0 }},
		{name: "double decimal", kind: PrimitiveDouble, input: ".5", canonical: "0.5", want: func(v float64) bool { return v == 0.5 }},
		{name: "float infinity", kind: PrimitiveFloat, input: "INF", canonical: "INF", want: func(v float64) bool { return math.IsInf(v, 1) }},
		{name: "double nan", kind: PrimitiveDouble, input: "NaN", canonical: "NaN", want: math.IsNaN},
		{name: "range overflow remains valid", kind: PrimitiveDouble, input: "1E9999", canonical: "INF", want: func(v float64) bool { return math.IsInf(v, 1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseFloatValue(tt.kind, tt.input, PrimitiveNeedCanonical)
			if err != nil {
				t.Fatalf("ParseFloatValue() error = %v", err)
			}
			if got.Canonical != tt.canonical || !tt.want(got.Value) {
				t.Fatalf("ParseFloatValue() = %+v, want canonical=%q", got, tt.canonical)
			}
		})
	}
}

func TestParseFloatValueRejectsInvalidLexicalAndPrimitive(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "+", ".", "+.", "1e", "1e+", "+INF", "nan", "1 2"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseFloatValue(PrimitiveDouble, input, PrimitiveNeedCanonical); err == nil || err.Error() != "invalid float" {
				t.Fatalf("ParseFloatValue(%q) error = %v, want invalid float", input, err)
			}
		})
	}
	if _, err := ParseFloatValue(PrimitiveString, "1", PrimitiveNeedCanonical); err == nil || err.Error() != "invalid float primitive" {
		t.Fatalf("ParseFloatValue() error = %v, want invalid float primitive", err)
	}
}

func TestFloatValueRelations(t *testing.T) {
	t.Parallel()

	if !EqualFloatValues(math.NaN(), math.NaN()) {
		t.Fatal("EqualFloatValues(NaN, NaN) = false")
	}
	if got := FloatRelation(math.NaN(), math.NaN()); got != OrderedFacetEqual {
		t.Fatalf("FloatRelation(NaN, NaN) = %v, want equal", got)
	}
	if got := FloatBoundsRelation(math.NaN(), math.NaN()); got != OrderedFacetIncomparable {
		t.Fatalf("FloatBoundsRelation(NaN, NaN) = %v, want incomparable", got)
	}
}

func TestValidateFloatFacets(t *testing.T) {
	t.Parallel()

	bound := func(v float64) FloatFacetValue {
		return FloatFacetValue{Value: v, Present: true}
	}
	tests := []struct {
		name     string
		facets   FloatFacetValues
		value    float64
		wantErr  string
		wantMeta bool
	}{
		{name: "min inclusive", facets: FloatFacetValues{MinInclusive: bound(1), Facets: FacetMinInclusive}, value: 0.9, wantErr: "minInclusive facet failed"},
		{name: "max inclusive", facets: FloatFacetValues{MaxInclusive: bound(10), Facets: FacetMaxInclusive}, value: 10.1, wantErr: "maxInclusive facet failed"},
		{name: "min exclusive", facets: FloatFacetValues{MinExclusive: bound(1), Facets: FacetMinExclusive}, value: 1, wantErr: "minExclusive facet failed"},
		{name: "max exclusive", facets: FloatFacetValues{MaxExclusive: bound(10), Facets: FacetMaxExclusive}, value: 10, wantErr: "maxExclusive facet failed"},
		{name: "nan bound is not satisfied", facets: FloatFacetValues{MinInclusive: bound(math.NaN()), Facets: FacetMinInclusive}, value: math.NaN(), wantErr: "minInclusive facet failed"},
		{name: "missing metadata", facets: FloatFacetValues{Facets: FacetMinInclusive}, value: 1, wantMeta: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateFloatFacets(tt.facets, tt.value)
			if tt.wantMeta {
				if !errors.Is(err, ErrSimpleValueMetadata) {
					t.Fatalf("ValidateFloatFacets() error = %v, want metadata error", err)
				}
				return
			}
			if got := errorMessage(err); got != tt.wantErr {
				t.Fatalf("ValidateFloatFacets() error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestFloatFacetBoundsAndRestrictions(t *testing.T) {
	t.Parallel()

	bound := func(v float64) FloatFacetValue {
		return FloatFacetValue{Value: v, Present: true}
	}
	if err := ValidateFloatFacetBounds(PrimitiveDouble, FloatFacetValues{
		MinInclusive: bound(math.NaN()),
		MaxInclusive: bound(math.NaN()),
		Facets:       FacetMinInclusive | FacetMaxInclusive,
	}); errorMessage(err) != "float lower bound cannot exceed upper bound" {
		t.Fatalf("ValidateFloatFacetBounds() error = %v, want NaN bounds rejected", err)
	}
	if FloatOrderedFacetsRestrict(
		FloatFacetValues{MinInclusive: bound(0), Facets: FacetMinInclusive},
		FloatFacetValues{MinInclusive: bound(1), Facets: FacetMinInclusive},
	) {
		t.Fatal("FloatOrderedFacetsRestrict() accepted lower derived minInclusive")
	}
	if !FloatOrderedFacetsRestrict(
		FloatFacetValues{MinInclusive: bound(1), Facets: FacetMinInclusive},
		FloatFacetValues{MinInclusive: bound(1), Facets: FacetMinInclusive},
	) {
		t.Fatal("FloatOrderedFacetsRestrict() rejected equal minInclusive")
	}
}
