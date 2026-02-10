package schemafacet

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
)

func TestCompareDateTimeValuesPrimitiveKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		typeName  string
		left      string
		right     string
		wantOrder int
	}{
		{name: "date", typeName: "date", left: "2001-01-01Z", right: "2001-01-02Z", wantOrder: -1},
		{name: "dateTime", typeName: "dateTime", left: "2001-01-01T00:00:00Z", right: "2001-01-02T00:00:00Z", wantOrder: -1},
		{name: "time", typeName: "time", left: "01:00:00Z", right: "02:00:00Z", wantOrder: -1},
		{name: "gYear", typeName: "gYear", left: "2001", right: "2002", wantOrder: -1},
		{name: "gYearMonth", typeName: "gYearMonth", left: "2001-01", right: "2001-02", wantOrder: -1},
		{name: "gMonth", typeName: "gMonth", left: "--01", right: "--02", wantOrder: -1},
		{name: "gMonthDay", typeName: "gMonthDay", left: "--01-01", right: "--01-02", wantOrder: -1},
		{name: "gDay", typeName: "gDay", left: "---01", right: "---02", wantOrder: -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := compareDateTimeValues(tc.left, tc.right, tc.typeName)
			if err != nil {
				t.Fatalf("compareDateTimeValues(%q, %q, %q) error = %v", tc.left, tc.right, tc.typeName, err)
			}
			if got != tc.wantOrder {
				t.Fatalf("compareDateTimeValues(%q, %q, %q) = %d, want %d", tc.left, tc.right, tc.typeName, got, tc.wantOrder)
			}
		})
	}
}

func TestCompareDateTimeValuesUnknownTypeFallsBackToLexical(t *testing.T) {
	t.Parallel()

	got, err := compareDateTimeValues("b", "a", "unknown-type")
	if err != nil {
		t.Fatalf("compareDateTimeValues() error = %v", err)
	}
	if got != 1 {
		t.Fatalf("compareDateTimeValues() = %d, want 1", got)
	}
}

func TestCompareFacetValuesBuiltinAndSimpleParity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		typeName  builtins.TypeName
		left      string
		right     string
		wantOrder int
		wantErr   error
	}{
		{name: "numeric int", typeName: builtins.TypeNameInt, left: "2", right: "10", wantOrder: -1},
		{name: "date", typeName: builtins.TypeNameDate, left: "2001-01-01Z", right: "2001-01-02Z", wantOrder: -1},
		{name: "float NaN", typeName: builtins.TypeNameFloat, left: "NaN", right: "1", wantErr: ErrFloatNotComparable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			builtinType := builtins.Get(tc.typeName)
			if builtinType == nil {
				t.Fatalf("missing builtin type %s", tc.typeName)
			}
			simpleType, err := builtins.NewSimpleType(tc.typeName)
			if err != nil {
				t.Fatalf("NewSimpleType(%s) error = %v", tc.typeName, err)
			}

			gotBuiltin, errBuiltin := CompareFacetValues(tc.left, tc.right, builtinType)
			gotSimple, errSimple := CompareFacetValues(tc.left, tc.right, simpleType)

			if tc.wantErr != nil {
				if !errors.Is(errBuiltin, tc.wantErr) {
					t.Fatalf("builtin CompareFacetValues() error = %v, want %v", errBuiltin, tc.wantErr)
				}
				if !errors.Is(errSimple, tc.wantErr) {
					t.Fatalf("simple CompareFacetValues() error = %v, want %v", errSimple, tc.wantErr)
				}
				return
			}

			if errBuiltin != nil {
				t.Fatalf("builtin CompareFacetValues() error = %v", errBuiltin)
			}
			if errSimple != nil {
				t.Fatalf("simple CompareFacetValues() error = %v", errSimple)
			}
			if gotBuiltin != tc.wantOrder {
				t.Fatalf("builtin CompareFacetValues() = %d, want %d", gotBuiltin, tc.wantOrder)
			}
			if gotSimple != tc.wantOrder {
				t.Fatalf("simple CompareFacetValues() = %d, want %d", gotSimple, tc.wantOrder)
			}
			if gotBuiltin != gotSimple {
				t.Fatalf("builtin/simple CompareFacetValues() mismatch: %d vs %d", gotBuiltin, gotSimple)
			}
		})
	}
}
