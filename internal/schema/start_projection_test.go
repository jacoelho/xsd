package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestTypeInfoProjection(t *testing.T) {
	t.Parallel()

	info := newTypeInfo(typeInfoShape{Block: DerivationRestriction, Abstract: true})
	if info.Block != DerivationRestriction || !info.Abstract {
		t.Fatalf("NewTypeInfo() = %+v, want projected facts", info)
	}
	if info == newTypeInfo(typeInfoShape{Block: DerivationRestriction}) {
		t.Fatal("type info values match despite different abstract flag")
	}
}

func TestSchemaTypeInfoClassifiesUnavailableSimpleDependencies(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{})
	availableList, err := builder.Add(value.TypeSpec{
		Variety:           value.List,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.BuiltinType(value.PrimitiveString),
	})
	if err != nil {
		t.Fatalf("value.Builder.Add() error = %v", err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatalf("value.Builder.Seal() error = %v", err)
	}
	missing := value.BuiltinTypeCount + 1
	unavailableList := value.BuiltinTypeCount + 2
	unavailable := make([]bool, int(unavailableList)+1)
	unavailable[missing] = true
	unavailable[unavailableList] = true
	rt := &Schema{program: schemaProgram{
		Value:                 program,
		SimpleTypeUnavailable: unavailable,
		ComplexTypes: []complexTypeRead{
			{textType: unavailableList, flags: complexTypeReadSimple},
			{textType: missing, flags: complexTypeReadSimple},
		},
	}}

	for _, id := range []TypeID{SimpleRef(missing), SimpleRef(unavailableList), ComplexRef(0)} {
		info, ok := rt.TypeInfo(id)
		if !ok || !info.Unavailable {
			t.Fatalf("TypeInfo(%+v) = %+v, %v; want unavailable", id, info, ok)
		}
	}
	for _, id := range []SimpleTypeID{missing, unavailableList} {
		unavailable, ok := rt.SimpleTypeUnavailable(id)
		if !ok || !unavailable {
			t.Fatalf("SimpleTypeUnavailable(%d) = %v, %v; want true, true", id, unavailable, ok)
		}
	}
	available := value.BuiltinType(value.PrimitiveString)
	if unavailable, ok := rt.SimpleTypeUnavailable(available); !ok || unavailable {
		t.Fatalf("SimpleTypeUnavailable(available) = %v, %v", unavailable, ok)
	}
	invalid := unavailableList + 1
	if _, ok := rt.SimpleTypeUnavailable(invalid); ok {
		t.Fatal("SimpleTypeUnavailable(invalid) succeeded")
	}
	for _, id := range []TypeID{SimpleRef(available), SimpleRef(availableList)} {
		info, ok := rt.TypeInfo(id)
		if !ok || info.Unavailable {
			t.Fatalf("TypeInfo(%+v) = %+v, %v; want available", id, info, ok)
		}
	}
	if _, ok := rt.TypeInfo(SimpleRef(invalid)); ok {
		t.Fatal("TypeInfo(invalid simple ID) succeeded")
	}
	if info, ok := rt.TypeInfo(ComplexRef(1)); !ok || !info.Unavailable {
		t.Fatalf("TypeInfo(complex type with direct missing text type) = %+v, %v; want unavailable", info, ok)
	}
}
