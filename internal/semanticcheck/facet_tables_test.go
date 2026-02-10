package semanticcheck

import (
	"testing"

	facetengine "github.com/jacoelho/xsd/internal/schemafacet"
)

func TestIsValidFacetName(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{name: "length", valid: true},
		{name: "minInclusive", valid: true},
		{name: "fractionDigits", valid: true},
		{name: "unknownFacet", valid: false},
	}
	for _, tc := range cases {
		if got := facetengine.IsValidFacetName(tc.name); got != tc.valid {
			t.Fatalf("facet %q valid=%v, want %v", tc.name, got, tc.valid)
		}
	}
}

func TestBuiltinRangeFacetInfoFor(t *testing.T) {
	info, ok := builtinRangeFacetInfoFor("positiveInteger")
	if !ok {
		t.Fatal("expected builtin range info for positiveInteger")
	}
	if !info.hasMin || info.minValue != "1" || !info.minInclusive {
		t.Fatalf("unexpected positiveInteger min info: %+v", info)
	}
	if info.hasMax {
		t.Fatalf("unexpected positiveInteger max info: %+v", info)
	}

	info, ok = builtinRangeFacetInfoFor("unsignedInt")
	if !ok {
		t.Fatal("expected builtin range info for unsignedInt")
	}
	if !info.hasMin || info.minValue != "0" || !info.minInclusive {
		t.Fatalf("unexpected unsignedInt min info: %+v", info)
	}
	if !info.hasMax || info.maxValue != "4294967295" || !info.maxInclusive {
		t.Fatalf("unexpected unsignedInt max info: %+v", info)
	}

	if _, ok := builtinRangeFacetInfoFor("string"); ok {
		t.Fatal("expected no builtin range info for string")
	}
}
