package runtime

import (
	"errors"
	"math"
	"testing"
)

func TestParsePrimitiveActual(t *testing.T) {
	tests := []struct {
		name          string
		kind          PrimitiveKind
		input         string
		needs         PrimitiveValueNeed
		wantCanonical string
		check         func(*testing.T, PrimitiveActualValue)
	}{
		{
			name:          "string",
			kind:          PrimitiveString,
			input:         "a\u00e9",
			needs:         PrimitiveNeedCanonical | PrimitiveNeedLength,
			wantCanonical: "a\u00e9",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveString || !actual.Valid || actual.Length != 2 {
					t.Fatalf("actual = %+v, want valid string length 2", actual)
				}
			},
		},
		{
			name:          "boolean",
			kind:          PrimitiveBoolean,
			input:         "1",
			wantCanonical: "true",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveBoolean || !actual.Valid || !actual.Boolean {
					t.Fatalf("actual = %+v, want true boolean", actual)
				}
			},
		},
		{
			name:          "decimal",
			kind:          PrimitiveDecimal,
			input:         "01.20",
			needs:         PrimitiveNeedCanonical,
			wantCanonical: "1.2",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveDecimal || !actual.Valid || actual.Decimal.CanonicalText() != "1.2" {
					t.Fatalf("actual = %+v, want decimal 1.2", actual)
				}
			},
		},
		{
			name:          "double nan",
			kind:          PrimitiveDouble,
			input:         "NaN",
			needs:         PrimitiveNeedCanonical,
			wantCanonical: "NaN",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveDouble || !actual.Valid || !math.IsNaN(actual.Float) {
					t.Fatalf("actual = %+v, want double NaN", actual)
				}
			},
		},
		{
			name:          "duration actual",
			kind:          PrimitiveDuration,
			input:         "P1D",
			wantCanonical: "P1D",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveDuration || !actual.Valid {
					t.Fatalf("actual = %+v, want valid duration", actual)
				}
			},
		},
		{
			name:          "date actual",
			kind:          PrimitiveDate,
			input:         "2000-01-01Z",
			needs:         PrimitiveNeedCanonical,
			wantCanonical: "2000-01-01Z",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveDate || !actual.Valid {
					t.Fatalf("actual = %+v, want valid date", actual)
				}
			},
		},
		{
			name:          "dateTime",
			kind:          PrimitiveDateTime,
			input:         "2000-01-01T00:00:00Z",
			needs:         PrimitiveNeedCanonical,
			wantCanonical: "2000-01-01T00:00:00Z",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveDateTime || !actual.Valid {
					t.Fatalf("actual = %+v, want valid dateTime", actual)
				}
			},
		},
		{
			name:          "time",
			kind:          PrimitiveTime,
			input:         "12:00:00Z",
			needs:         PrimitiveNeedCanonical,
			wantCanonical: "12:00:00Z",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveTime || !actual.Valid {
					t.Fatalf("actual = %+v, want valid time", actual)
				}
			},
		},
		{
			name:          "gMonth actual",
			kind:          PrimitiveGMonth,
			input:         "--05Z",
			needs:         PrimitiveNeedCanonical,
			wantCanonical: "--05Z",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveGMonth || !actual.Valid {
					t.Fatalf("actual = %+v, want valid gMonth", actual)
				}
			},
		},
		{
			name:          "hexBinary",
			kind:          PrimitiveHexBinary,
			input:         "0aff",
			needs:         PrimitiveNeedCanonical | PrimitiveNeedLength,
			wantCanonical: "0AFF",
			check: func(t *testing.T, actual PrimitiveActualValue) {
				t.Helper()
				if actual.Kind != PrimitiveHexBinary || !actual.Valid || actual.Length != 2 {
					t.Fatalf("actual = %+v, want hexBinary length 2", actual)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParsePrimitiveActual(test.kind, test.input, test.needs)
			if err != nil {
				t.Fatalf("ParsePrimitiveActual() error = %v", err)
			}
			if got.Canonical != test.wantCanonical {
				t.Fatalf("ParsePrimitiveActual().Canonical = %q, want %q", got.Canonical, test.wantCanonical)
			}
			test.check(t, got.Actual)
		})
	}
}

func TestParsePrimitiveActualLazyCanonical(t *testing.T) {
	for _, tt := range []struct {
		name  string
		kind  PrimitiveKind
		input string
	}{
		{name: "date lazy", kind: PrimitiveDate, input: "2000-01-01Z"},
		{name: "dateTime", kind: PrimitiveDateTime, input: "2000-01-01T00:00:00Z"},
		{name: "time", kind: PrimitiveTime, input: "12:00:00Z"},
		{name: "gMonth lazy", kind: PrimitiveGMonth, input: "--05Z"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePrimitiveActual(tt.kind, tt.input, 0)
			if err != nil {
				t.Fatalf("ParsePrimitiveActual() error = %v", err)
			}
			if got.Canonical != "" {
				t.Fatalf("ParsePrimitiveActual().Canonical = %q, want empty without canonical need", got.Canonical)
			}
			if !got.Actual.Valid || got.Actual.Kind != tt.kind {
				t.Fatalf("ParsePrimitiveActual().Actual = %+v, want valid kind %v", got.Actual, tt.kind)
			}
		})
	}
}

func TestParsePrimitiveActualErrors(t *testing.T) {
	if _, err := ParsePrimitiveActual(PrimitiveAnyURI, ":a", PrimitiveNeedCanonical); err == nil || err.Error() != "invalid anyURI" {
		t.Fatalf("ParsePrimitiveActual(anyURI) error = %v, want invalid anyURI", err)
	}
	for _, kind := range []PrimitiveKind{PrimitiveQName, PrimitiveNotation, PrimitiveKind(255)} {
		if _, err := ParsePrimitiveActual(kind, "value", PrimitiveNeedCanonical); !errors.Is(err, ErrSimpleValueMetadata) {
			t.Fatalf("ParsePrimitiveActual(%v) error = %v, want metadata sentinel", kind, err)
		}
	}
}

func TestEqualPrimitiveActualValues(t *testing.T) {
	decimalA, err := ParseDecimalValue("1.0")
	if err != nil {
		t.Fatal(err)
	}
	decimalB, err := ParseDecimalValue("1.00")
	if err != nil {
		t.Fatal(err)
	}
	duration, err := ParseDurationValue("P1D")
	if err != nil {
		t.Fatal(err)
	}
	date, err := ParseDateValue("2000-01-01Z")
	if err != nil {
		t.Fatal(err)
	}
	time, err := ParseTimeValue("12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	gMonthA, err := ParseGValue(PrimitiveGMonth, "--05Z")
	if err != nil {
		t.Fatal(err)
	}
	gMonthB, err := ParseGValue(PrimitiveGMonth, "--05+00:00")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name             string
		actual           PrimitiveActualValue
		canonical        string
		literal          PrimitiveActualValue
		literalCanonical string
		want             bool
	}{
		{
			name:             "canonical fallback",
			canonical:        "abc",
			literalCanonical: "abc",
			want:             true,
		},
		{
			name:             "kind mismatch uses canonical fallback",
			actual:           PrimitiveActualValue{Kind: PrimitiveBoolean, Valid: true, Boolean: true},
			canonical:        "true",
			literal:          PrimitiveActualValue{Kind: PrimitiveString, Valid: true},
			literalCanonical: "true",
			want:             true,
		},
		{
			name:             "boolean",
			actual:           PrimitiveActualValue{Kind: PrimitiveBoolean, Valid: true, Boolean: true},
			literal:          PrimitiveActualValue{Kind: PrimitiveBoolean, Valid: true, Boolean: true},
			literalCanonical: "false",
			want:             true,
		},
		{
			name:             "decimal value",
			actual:           PrimitiveActualValue{Kind: PrimitiveDecimal, Valid: true, Decimal: decimalA},
			literal:          PrimitiveActualValue{Kind: PrimitiveDecimal, Valid: true, Decimal: decimalB},
			literalCanonical: "2",
			want:             true,
		},
		{
			name:             "float nan",
			actual:           PrimitiveActualValue{Kind: PrimitiveFloat, Valid: true, Float: math.NaN()},
			literal:          PrimitiveActualValue{Kind: PrimitiveFloat, Valid: true, Float: math.NaN()},
			literalCanonical: "0",
			want:             true,
		},
		{
			name:             "duration",
			actual:           PrimitiveActualValue{Kind: PrimitiveDuration, Valid: true, Duration: duration},
			literal:          PrimitiveActualValue{Kind: PrimitiveDuration, Valid: true, Duration: duration},
			literalCanonical: "P2D",
			want:             true,
		},
		{
			name:             "temporal date",
			actual:           PrimitiveActualValue{Kind: PrimitiveDate, Valid: true, Temporal: date.Temporal()},
			literal:          PrimitiveActualValue{Kind: PrimitiveDate, Valid: true, Temporal: date.Temporal()},
			literalCanonical: "2001-01-01Z",
			want:             true,
		},
		{
			name:             "time",
			actual:           PrimitiveActualValue{Kind: PrimitiveTime, Valid: true, Time: time},
			literal:          PrimitiveActualValue{Kind: PrimitiveTime, Valid: true, Time: time},
			literalCanonical: "13:00:00Z",
			want:             true,
		},
		{
			name:             "g value",
			actual:           PrimitiveActualValue{Kind: PrimitiveGMonth, Valid: true, G: gMonthA},
			literal:          PrimitiveActualValue{Kind: PrimitiveGMonth, Valid: true, G: gMonthB},
			literalCanonical: "--06Z",
			want:             true,
		},
		{
			name:             "boolean mismatch",
			actual:           PrimitiveActualValue{Kind: PrimitiveBoolean, Valid: true, Boolean: true},
			literal:          PrimitiveActualValue{Kind: PrimitiveBoolean, Valid: true, Boolean: false},
			literalCanonical: "true",
			want:             false,
		},
	}
	for _, test := range tests {
		if got := EqualPrimitiveActualValues(test.actual, test.canonical, test.literal, test.literalCanonical); got != test.want {
			t.Fatalf("%s: EqualPrimitiveActualValues() = %v, want %v", test.name, got, test.want)
		}
	}
}
