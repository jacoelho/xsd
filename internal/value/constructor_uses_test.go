package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestConstructorUsesAvailabilityAndLifetime(t *testing.T) {
	t.Parallel()
	assertUses := func(view value.ConstructorUses, want ...value.TypeID) {
		t.Helper()
		if view.Len() != len(want) {
			t.Fatalf("Len() = %d; want %d", view.Len(), len(want))
		}
		for i, id := range want {
			if got, ok := view.At(i); !ok || got != id {
				t.Fatalf("At(%d) = %d, %t; want %d, true", i, got, ok, id)
			}
		}
		for _, i := range []int{-1, len(want)} {
			if _, ok := view.At(i); ok {
				t.Fatalf("At(%d) accepted an out-of-bounds index", i)
			}
		}
	}
	assertUses(value.ConstructorUses{})
	builder := value.NewBuilder(value.BuilderOptions{})
	anySimple := normalizedBuiltin(t, "anySimpleType")
	boolean := normalizedBuiltin(t, "boolean")
	reserved, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		builder *value.Builder
		id      value.TypeID
	}{
		{nil, anySimple}, {new(value.Builder), anySimple},
		{builder, value.NoType}, {builder, reserved}, {builder, reserved + 1},
	} {
		if view, ok := test.builder.ConstructorUses(test.id); ok || view.Len() != 0 {
			t.Fatalf("unavailable ConstructorUses(%d) = %#v, %t", test.id, view, ok)
		}
	}
	atomic, ok := builder.ConstructorUses(anySimple)
	if !ok {
		t.Fatal("built-in atomic definition unavailable")
	}
	assertUses(atomic)
	if err := builder.Complete(reserved, value.TypeSpec{
		Variety: value.List, Primitive: value.PrimitiveString,
		Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
		Base: value.NoType, ListItem: anySimple,
	}); err != nil {
		t.Fatal(err)
	}
	list, ok := builder.ConstructorUses(reserved)
	if !ok {
		t.Fatal("completed list unavailable")
	}
	assertUses(list, anySimple)
	members := []value.TypeID{boolean, anySimple, boolean}
	unionID := completeNormalizedType(t, builder, value.TypeSpec{
		Variety: value.Union, Primitive: value.PrimitiveString,
		Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
		Base: value.NoType, ListItem: value.NoType, Union: members,
	})
	union, ok := builder.ConstructorUses(unionID)
	if !ok {
		t.Fatal("completed union unavailable")
	}
	members[0] = anySimple
	assertUses(union, boolean, anySimple, boolean)
	for range 256 {
		completeNormalizedType(t, builder, atomicFacetType(value.PrimitiveString, value.FacetSpec{}))
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatal(err)
	}
	assertUses(list, anySimple)
	assertUses(union, boolean, anySimple, boolean)
	for _, id := range []value.TypeID{anySimple, reserved, unionID} {
		if _, ok := builder.ConstructorUses(id); ok {
			t.Fatalf("sealed builder exposed ConstructorUses(%d)", id)
		}
	}
}

func TestConstructorUsesExcludesProvisionalCompletion(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	observed := false
	resolver := value.Resolver{QName: func(string) (value.ExpandedName, bool) {
		observed = true
		if _, ok := builder.TypeView(id); !ok {
			t.Fatal("provisional TypeView unavailable")
		}
		if _, ok := builder.ConstructorUses(id); ok {
			t.Fatal("provisional definition exposed constructor uses")
		}
		return value.ExpandedName{}, false
	}}
	spec := atomicFacetType(value.PrimitiveQName, value.FacetSpec{
		Enumeration: []value.LiteralSpec{{Type: value.NoType, Lexical: "p:item", Resolver: resolver}},
	})
	if err := builder.Complete(id, spec); err == nil || !observed {
		t.Fatalf("Complete() = %v, resolver called = %t; want observed failure", err, observed)
	}
	if _, ok := builder.ConstructorUses(id); ok {
		t.Fatal("failed completion exposed constructor uses")
	}
	spec.Facets.Enumeration[0].Resolver = value.Resolver{QName: func(string) (value.ExpandedName, bool) {
		return value.ExpandedName{Namespace: "urn:test", Local: "item"}, true
	}}
	if err := builder.Complete(id, spec); err != nil {
		t.Fatal(err)
	}
	if view, ok := builder.ConstructorUses(id); !ok || view.Len() != 0 {
		t.Fatalf("committed atomic constructor uses = %#v, %t; want empty, true", view, ok)
	}
}
