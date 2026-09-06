package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestCompositeValueIdentityProjections(t *testing.T) {
	for _, tc := range []struct {
		name, member string
		id, ref      string
	}{
		{name: "ID union", member: "ID", id: "item"},
		{name: "IDREF union", member: "IDREF", ref: "item"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{})
			member, _ := value.BuiltinTypeID(tc.member)
			integer, _ := value.BuiltinTypeID("int")
			union, err := builder.Add(value.TypeSpec{Variety: value.Union, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{member, integer}})
			if err != nil {
				t.Fatal(err)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
				got, err := program.Validate(union, "item", value.Resolver{}, needs, 16<<20, nil)
				if err != nil {
					t.Fatal(err)
				}
				if got.IDs() != tc.id || got.IDRefs() != tc.ref {
					t.Errorf("needs %d: IDs=%q, IDRefs=%q; want %q, %q", needs, got.IDs(), got.IDRefs(), tc.id, tc.ref)
				}
			}
		})
	}
}

func TestListIdentityUsesSharedFraming(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Add(value.TypeSpec{Variety: value.List, Base: value.NoType, ListItem: value.BuiltinType(value.PrimitiveAnyURI)})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := program.Validate(id, "urn:first urn:second", value.Resolver{}, value.NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := value.ListIdentityKey([]string{value.PrimitiveIdentityKey(value.PrimitiveAnyURI, "urn:first"), value.PrimitiveIdentityKey(value.PrimitiveAnyURI, "urn:second")})
	if got.IdentityKey() != want {
		t.Fatalf("list identity=%q, want same framing as XSI list %q", got.IdentityKey(), want)
	}
}

func TestListOfUnionCarriesSelectedIDREFs(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	idref, _ := value.BuiltinTypeID("IDREF")
	integer, _ := value.BuiltinTypeID("int")
	member, err := builder.Add(value.TypeSpec{
		Variety: value.Union,
		Base:    value.NoType, ListItem: value.NoType,
		Union: []value.TypeID{integer, idref},
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := builder.Add(value.TypeSpec{
		Variety: value.List, Base: value.NoType, ListItem: member,
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
		got, err := program.Validate(list, "one 2 three", value.Resolver{}, needs, 16<<20, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.IDRefs() != "one three" {
			t.Fatalf("needs %d: IDRefs=%q, want %q", needs, got.IDRefs(), "one three")
		}
	}
}
