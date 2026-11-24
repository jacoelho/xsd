package value

import "testing"

func TestSealedInputRequirementsMatchTypeDefinitions(t *testing.T) {
	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	define := func(spec TypeSpec) TypeID {
		t.Helper()
		id, err := builder.Reserve()
		if err != nil {
			t.Fatal(err)
		}
		if err := builder.Complete(id, spec); err != nil {
			t.Fatal(err)
		}
		return id
	}
	plainString := define(TypeSpec{Variety: Atomic, Primitive: PrimitiveString, Whitespace: WhitespacePreserve, WhitespacePresent: true, Base: builtinString, ListItem: NoType})
	boundedString := define(TypeSpec{Variety: Atomic, Primitive: PrimitiveString, Whitespace: WhitespacePreserve, WhitespacePresent: true, Base: builtinString, ListItem: NoType,
		Facets: FacetSpec{MaxLength: CardinalityFacet{Present: true, Value: 3}}})
	refList := define(TypeSpec{Variety: List, Primitive: PrimitiveString, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: AnySimpleTypeID, ListItem: builtinIDREF})
	qnameUnion := define(TypeSpec{Variety: Union, Base: AnySimpleTypeID, ListItem: NoType, Union: []TypeID{builtinString, builtinQName}})
	program, err := sealAll(builder)
	if err != nil {
		t.Fatal(err)
	}
	want := map[TypeID]InputRequirements{
		builtinString: {LexicalCanonical: true, UnconstrainedString: true},
		builtinToken:  {},
		builtinID:     {DocumentIdentity: true, CarriesID: true},
		builtinIDREFS: {DocumentIdentity: true},
		builtinQName:  {NeedsQName: true},
		plainString:   {LexicalCanonical: true, UnconstrainedString: true},
		boundedString: {LexicalCanonical: true},
		refList:       {DocumentIdentity: true},
		qnameUnion:    {NeedsQName: true},
	}
	for id, expected := range want {
		got, ok := program.InputRequirements(id)
		if !ok || got != expected {
			t.Errorf("InputRequirements(%d) = %+v, %v; want %+v", id, got, ok, expected)
		}
	}
	for id := TypeID(0); id < qnameUnion+1; id++ {
		def, _ := program.typeDef(id)
		got, ok := program.InputRequirements(id)
		if !ok || got != inputRequirements(def) {
			t.Errorf("InputRequirements(%d) = %+v, want recomputed %+v", id, got, inputRequirements(def))
		}
	}
}
