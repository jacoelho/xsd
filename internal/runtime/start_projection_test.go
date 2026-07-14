package runtime

import "testing"

func TestElementStartInfoProjection(t *testing.T) {
	t.Parallel()

	info := NewElementStartInfo(ElementStartInfoShape{
		Type:     SimpleRef(1),
		Block:    DerivationExtension,
		Abstract: true,
		Nillable: true,
		Fixed:    true,
	})
	if info.Type != SimpleRef(1) || info.Block != DerivationExtension ||
		!info.Abstract || !info.Nillable || !info.Fixed {
		t.Fatalf("NewElementStartInfo() = %+v, want projected facts", info)
	}
	same := NewElementStartInfo(ElementStartInfoShape{
		Type: SimpleRef(1), Block: DerivationExtension, Abstract: true, Nillable: true, Fixed: true,
	})
	if !EqualElementStartInfo(info, same) {
		t.Fatal("EqualElementStartInfo() = false, want true")
	}
}

func TestTypeInfoProjection(t *testing.T) {
	t.Parallel()

	info := NewTypeInfo(TypeInfoShape{Block: DerivationRestriction, Abstract: true})
	if info.Block != DerivationRestriction || !info.Abstract {
		t.Fatalf("NewTypeInfo() = %+v, want projected facts", info)
	}
	if info == NewTypeInfo(TypeInfoShape{Block: DerivationRestriction}) {
		t.Fatal("type info values match despite different abstract flag")
	}
}

func TestSchemaTypeInfoClassifiesUnavailableSimpleDependencies(t *testing.T) {
	t.Parallel()

	types := []SimpleType{
		{Missing: true},
		{Base: NoSimpleType, ListItem: NoSimpleType, Variety: SimpleVarietyAtomic},
		{Base: NoSimpleType, ListItem: 0, Variety: SimpleVarietyList},
		{Base: NoSimpleType, ListItem: 1, Variety: SimpleVarietyList},
	}
	rt := &Schema{runtime: schemaRuntime{
		SimpleValueRoutes: newSimpleValueRouteReadsForSimpleTypes(types),
		ComplexTypes: []complexTypeRead{
			{textType: 2, flags: complexTypeReadSimple},
			{textType: 0, flags: complexTypeReadSimple},
		},
	}}

	for _, id := range []TypeID{SimpleRef(0), SimpleRef(2), ComplexRef(0)} {
		info, ok := rt.TypeInfo(id)
		if !ok || !info.Unavailable {
			t.Fatalf("TypeInfo(%+v) = %+v, %v; want unavailable", id, info, ok)
		}
	}
	for _, id := range []SimpleTypeID{0, 2} {
		unavailable, ok := rt.SimpleTypeUnavailable(id)
		if !ok || !unavailable {
			t.Fatalf("SimpleTypeUnavailable(%d) = %v, %v; want true, true", id, unavailable, ok)
		}
	}
	if unavailable, ok := rt.SimpleTypeUnavailable(1); !ok || unavailable {
		t.Fatalf("SimpleTypeUnavailable(available) = %v, %v", unavailable, ok)
	}
	if _, ok := rt.SimpleTypeUnavailable(4); ok {
		t.Fatal("SimpleTypeUnavailable(invalid) succeeded")
	}
	for _, id := range []TypeID{SimpleRef(1), SimpleRef(3)} {
		info, ok := rt.TypeInfo(id)
		if !ok || info.Unavailable {
			t.Fatalf("TypeInfo(%+v) = %+v, %v; want available", id, info, ok)
		}
	}
	if _, ok := rt.TypeInfo(SimpleRef(4)); ok {
		t.Fatal("TypeInfo(invalid simple ID) succeeded")
	}
	if _, ok := rt.TypeInfo(ComplexRef(1)); ok {
		t.Fatal("TypeInfo(complex type with direct missing text type) succeeded")
	}
}
