package schema

import (
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestRuntimePublicationCloneHelpersDoNotAliasMutableState(t *testing.T) {
	simpleDerivation := simpleTypeDerivation{Union: []SimpleTypeID{1, 2}}
	clonedSimpleDerivation := cloneSimpleTypeDerivation(simpleDerivation)
	simpleDerivation.Union[0] = 9
	if clonedSimpleDerivation.Union[0] != 1 {
		t.Fatalf("CloneSimpleTypeDerivation aliased union slice: %#v", clonedSimpleDerivation.Union)
	}

	valueConstraintIdentity := constraintIdentity{
		ResolvedNames: []resolvedValueName{{Lexical: "p:item"}},
	}
	clonedValueConstraintIdentity := cloneValueConstraintIdentity(valueConstraintIdentity)
	valueConstraintIdentity.ResolvedNames[0].Lexical = "p:other"
	if clonedValueConstraintIdentity.ResolvedNames[0].Lexical != "p:item" {
		t.Fatalf("CloneValueConstraintIdentity aliased resolved-name slice: %#v", clonedValueConstraintIdentity.ResolvedNames)
	}
}

func TestValueConstraintSimpleTypeOwnsUnionCopy(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.Complete(id, value.TypeSpec{
		Variety: value.Union, Primitive: value.PrimitiveString,
		Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
		Base: value.NoType, ListItem: value.NoType,
		Union: []SimpleTypeID{value.BuiltinType(value.PrimitiveBoolean), value.BuiltinType(value.PrimitiveDecimal)},
	}); err != nil {
		t.Fatal(err)
	}
	build := schemaBuild{valueBuilder: builder}
	want := []SimpleTypeID{value.BuiltinType(value.PrimitiveBoolean), value.BuiltinType(value.PrimitiveDecimal)}
	first, ok := build.ValueConstraintSimpleType(id)
	if !ok || !reflect.DeepEqual(first.Union, want) {
		t.Fatalf("ValueConstraintSimpleType() = %#v, %v; want boolean, decimal", first, ok)
	}
	first.Union[0] = value.BuiltinType(value.PrimitiveString)
	again, ok := build.ValueConstraintSimpleType(id)
	if !ok || !reflect.DeepEqual(again.Union, want) {
		t.Fatalf("ValueConstraintSimpleType() after mutation = %#v, %v; want boolean, decimal", again, ok)
	}
}
