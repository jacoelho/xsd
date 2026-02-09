package semanticcheck

import (
	"testing"

	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
)

func TestCompareFacetValuesDateTimeSpecials(t *testing.T) {
	bt := types.GetBuiltin(types.TypeNameDateTime)
	if bt == nil {
		t.Fatal("builtin dateTime not found")
	}

	cases := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{
			name: "24:00:00",
			v1:   "2000-01-01T24:00:00",
			v2:   "2000-01-02T00:00:00",
			want: 0,
		},
		{
			name: "leap second",
			v1:   "1999-12-31T23:59:60",
			v2:   "2000-01-01T00:00:00",
			want: -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := value.ParseDateTime([]byte(tc.v1)); err != nil {
				t.Fatalf("runtime parse %q: %v", tc.v1, err)
			}
			cmp, err := facetengine.CompareFacetValues(tc.v1, tc.v2, bt)
			if err != nil {
				t.Fatalf("facetengine.CompareFacetValues(%q, %q) error: %v", tc.v1, tc.v2, err)
			}
			if cmp != tc.want {
				t.Fatalf("facetengine.CompareFacetValues(%q, %q) = %d, want %d", tc.v1, tc.v2, cmp, tc.want)
			}
		})
	}
}

func TestCompareFacetValuesTimeSpecials(t *testing.T) {
	bt := types.GetBuiltin(types.TypeNameTime)
	if bt == nil {
		t.Fatal("builtin time not found")
	}

	cases := []string{
		"24:00:00",
		"23:59:60",
	}

	for _, val := range cases {
		t.Run(val, func(t *testing.T) {
			if _, err := value.ParseTime([]byte(val)); err != nil {
				t.Fatalf("runtime parse %q: %v", val, err)
			}
			cmp, err := facetengine.CompareFacetValues(val, val, bt)
			if err != nil {
				t.Fatalf("facetengine.CompareFacetValues(%q, %q) error: %v", val, val, err)
			}
			if cmp != 0 {
				t.Fatalf("facetengine.CompareFacetValues(%q, %q) = %d, want 0", val, val, cmp)
			}
		})
	}
}

func TestCompareFacetValuesDateTimeRejectsInvalidOffsets(t *testing.T) {
	bt := types.GetBuiltin(types.TypeNameDateTime)
	if bt == nil {
		t.Fatal("builtin dateTime not found")
	}

	invalid := []string{
		"2000-01-01T00:00:00+25:00",
		"2000-01-01T00:00:00+14:75",
	}

	for _, val := range invalid {
		t.Run(val, func(t *testing.T) {
			if _, err := value.ParseDateTime([]byte(val)); err == nil {
				t.Fatalf("runtime parse %q: expected error", val)
			}
			if _, err := facetengine.CompareFacetValues(val, val, bt); err == nil {
				t.Fatalf("facetengine.CompareFacetValues(%q) expected error", val)
			}
		})
	}
}
