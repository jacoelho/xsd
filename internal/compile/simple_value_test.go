package compile

import (
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestSimpleValueFacetCacheReusesCompletedTypeProjection(t *testing.T) {
	t.Parallel()

	types := make([]runtime.SimpleType, 4)
	types[3] = runtime.SimpleType{Facets: runtime.FacetSet{
		Enumeration: []runtime.CompiledLiteral{{Canonical: "a"}},
		Present:     runtime.FacetEnumeration,
	}}
	var cache simpleValueFacetCache
	first, ok := cache.read(types, 3)
	if !ok || len(first.Enumeration) != 1 {
		t.Fatalf("first facet cache read = %#v, %v", first, ok)
	}
	if len(cache.values) != 1 || cache.index[0] != 0 || cache.index[1] != 0 || cache.index[2] != 0 || cache.index[3] != 1 {
		t.Fatalf("sparse facet cache = index %v, values %d", cache.index, len(cache.values))
	}
	second, ok := cache.read(types, 3)
	if !ok || len(second.Enumeration) != 1 || &first.Enumeration[0] != &second.Enumeration[0] {
		t.Fatal("facet cache rebuilt a completed type projection")
	}
}

func TestSimpleValueFacetCachePoolsEnumerationBySourceIdentity(t *testing.T) {
	t.Parallel()

	shared := []runtime.CompiledLiteral{{Canonical: "a"}, {Canonical: "b"}}
	distinct := slices.Clone(shared)
	types := []runtime.SimpleType{
		{Facets: runtime.FacetSet{Enumeration: shared, Present: runtime.FacetEnumeration}},
		{Facets: runtime.FacetSet{Enumeration: shared, Present: runtime.FacetEnumeration}},
		{Facets: runtime.FacetSet{Enumeration: distinct, Present: runtime.FacetEnumeration}},
	}
	var cache simpleValueFacetCache
	first, _ := cache.read(types, 0)
	second, _ := cache.read(types, 1)
	third, _ := cache.read(types, 2)
	if &first.Enumeration[0] != &second.Enumeration[0] {
		t.Fatal("shared enumeration source did not reuse its projection")
	}
	if &first.Enumeration[0] == &third.Enumeration[0] {
		t.Fatal("distinct enumeration sources shared a projection by content")
	}
}

func TestSimpleValueFacetCacheSharedEnumerationAllocationsAreLinear(t *testing.T) {
	const count = 1_000
	enumeration := make([]runtime.CompiledLiteral, count)
	for i := range enumeration {
		enumeration[i].Canonical = "value"
	}
	types := make([]runtime.SimpleType, count)
	for i := range types {
		types[i].Facets = runtime.FacetSet{Enumeration: enumeration, Present: runtime.FacetEnumeration}
	}

	allocs := testing.AllocsPerRun(1, func() {
		var cache simpleValueFacetCache
		for id := range types {
			cache.read(types, runtime.SimpleTypeID(id))
		}
	})
	if allocs > 50 {
		t.Fatalf("facet cache allocations = %.0f, want at most 50", allocs)
	}
}
