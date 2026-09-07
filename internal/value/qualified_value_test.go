package value

import "testing"

func TestValueQualifiedNamesFollowAcceptedMembers(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	union, err := b.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveString,
		Union: []TypeID{builtinBoolean, builtinQName}, Base: NoType, ListItem: NoType,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := b.Add(TypeSpec{
		Variety: List, Primitive: PrimitiveString, Base: NoType, ListItem: union,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	resolver := Resolver{QName: func(lexical string) (ExpandedName, bool) {
		return ExpandedName{Namespace: "urn:test", Local: "item"}, lexical == "p:item"
	}}
	for _, test := range []struct {
		name    string
		lexical string
		typ     TypeID
		want    bool
	}{
		{name: "boolean member", typ: union, lexical: "true"},
		{name: "QName member", typ: union, lexical: "p:item", want: true},
		{name: "boolean list", typ: list, lexical: "true false"},
		{name: "mixed list", typ: list, lexical: "true p:item", want: true},
		{name: "empty list", typ: list, lexical: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, needs := range []Needs{0, NeedCanonical, NeedIdentity, NeedCanonical | NeedIdentity} {
				got, err := p.Validate(test.typ, test.lexical, resolver, needs, 16<<20, nil)
				if err != nil {
					t.Fatal(err)
				}
				if got.HasQualifiedNames() != test.want {
					t.Fatalf("value %q, needs %d: HasQualifiedNames = %v, want %v", test.lexical, needs, got.HasQualifiedNames(), test.want)
				}
			}
		})
	}
}
