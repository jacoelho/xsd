package schema

import (
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func TestValueConstraintNestedSharedUnionKeepsExactOwner(t *testing.T) {
	t.Parallel()
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	stringID := valuepkg.BuiltinType(valuepkg.PrimitiveString)
	makeUnion := func(members ...valuepkg.TypeID) valuepkg.TypeID {
		return completeConstraintFixtureType(t, builder, valuepkg.TypeSpec{Base: valuepkg.NoType, ListItem: valuepkg.NoType, Variety: valuepkg.Union, Union: members})
	}
	shared := makeUnion(stringID)
	left := makeUnion(shared)
	right := makeUnion(shared)
	outer := makeUnion(left, right)
	cached, err := builder.Validate(outer, "owner-value", valuepkg.Resolver{}, valuepkg.NeedCanonical|valuepkg.NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cached.Type() != outer || cached.CanonicalText() != "owner-value" {
		t.Fatalf("cached owner/canonical = %d/%q; want %d/owner-value", cached.Type(), cached.CanonicalText(), outer)
	}
	rt := &schemaBuild{valueBuilder: builder}
	if shapeErr := ValidateValueConstraintShape(rt, ValueConstraintValidation{Lexical: "owner-value", Canonical: "owner-value", Value: cached}, outer); shapeErr != nil {
		t.Fatalf("exact outer-owner cache rejected: %v", shapeErr)
	}
}
