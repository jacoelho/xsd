package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/source"
)

func TestCompiledListItemQueryBounds(t *testing.T) {
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	err = c.loadOwned([]source.Source{source.Bytes("query.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="inner"><xs:union memberTypes="xs:string xs:int"/></xs:simpleType>
  <xs:simpleType name="outer"><xs:union memberTypes="inner inner xs:boolean"/></xs:simpleType>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.index(); err != nil {
		t.Fatal(err)
	}
	if err := c.compileGlobals(); err != nil {
		t.Fatal(err)
	}
	q, ok := c.rt.Names.LookupQName("", "outer")
	if !ok {
		t.Fatal("compiled outer type missing")
	}
	id, ok := c.rt.GlobalTypes[q].Simple()
	if !ok {
		t.Fatal("outer is not a simple type")
	}
	allocs := testing.AllocsPerRun(1, func() {
		for range 10_000 {
			if c.simpleListItemReachesList(id) {
				t.Fatal("compiled scalar union unexpectedly contains a list")
			}
		}
	})
	if allocs > 10 {
		t.Fatalf("list-item query allocations = %.0f, want at most 10", allocs)
	}
	for _, invalid := range []SimpleTypeID{NoSimpleType, SimpleTypeID(1_000_000)} {
		if c.simpleListItemReachesList(invalid) {
			t.Fatalf("list-item query(%d) = true for invalid root", invalid)
		}
	}
}
