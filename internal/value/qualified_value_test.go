package value

import (
	"errors"
	"strings"
	"testing"
)

func TestValueQualifiedNamesFollowAcceptedMembers(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	union, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(union, TypeSpec{
		Variety: Union, Primitive: PrimitiveString,
		Union: []TypeID{builtinBoolean, builtinQName}, Base: NoType, ListItem: NoType,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(list, TypeSpec{
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

func TestProgramQualifiedPrimitiveIdentity(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	resolver := Resolver{
		QName:    func(string) (ExpandedName, bool) { return ExpandedName{Namespace: "urn:test", Local: "item"}, true },
		Notation: func(namespace, local string) bool { return namespace == "urn:test" && local == "item" },
	}
	qname, err := p.Validate(BuiltinType(PrimitiveQName), "p:item", resolver, NeedCanonical|NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	notation, err := p.Validate(BuiltinType(PrimitiveNotation), "p:item", resolver, NeedCanonical|NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if qname.CanonicalText() != "{urn:test}item" || notation.CanonicalText() != "{urn:test}item" {
		t.Fatalf("canonical names = %q, %q", qname.CanonicalText(), notation.CanonicalText())
	}
	if qname.IdentityKey() != "\x11\x1e{urn:test}item" || notation.IdentityKey() != "\x12\x1e{urn:test}item" {
		t.Fatalf("primitive identities = %q, %q", qname.IdentityKey(), notation.IdentityKey())
	}
	if qname.Equal(notation) || notation.Equal(qname) {
		t.Fatal("QName and NOTATION must have distinct value spaces")
	}
	if !qname.HasQualifiedNames() || !notation.HasQualifiedNames() {
		t.Fatal("accepted names must retain qualified-name flags")
	}
}

func TestProgramQualifiedUnionEnumeration(t *testing.T) {
	declared := Resolver{
		QName:    func(string) (ExpandedName, bool) { return ExpandedName{Namespace: "urn:test", Local: "item"}, true },
		Notation: func(namespace, local string) bool { return namespace == "urn:test" && local == "item" },
	}
	undeclared := declared
	undeclared.Notation = func(string, string) bool { return false }
	b := NewBuilder(BuilderOptions{})
	union, err := b.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	members := []TypeID{BuiltinType(PrimitiveNotation), BuiltinType(PrimitiveQName)}
	err = b.Complete(union, TypeSpec{
		Variety: Union, Primitive: PrimitiveString, Union: members,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	restricted, err := b.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	err = b.Complete(restricted, TypeSpec{
		Variety: Union, Primitive: PrimitiveString, Union: members,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: union, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{Lexical: "p:item", Type: NoType, Resolver: declared}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	fallback, err := p.Validate(union, "p:item", undeclared, NeedCanonical|NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Type() != union || fallback.SelectedType() != BuiltinType(PrimitiveQName) || !fallback.HasQualifiedNames() || fallback.CanonicalText() != "{urn:test}item" || fallback.IdentityKey() != "\x11\x1e{urn:test}item" {
		t.Fatalf("QName fallback = %#v", fallback)
	}
	_, err = p.Validate(restricted, "p:item", undeclared, NeedCanonical|NeedIdentity, 16<<20, nil)
	if !errors.Is(err, ErrFacet) {
		t.Fatalf("cross-kind enumeration = %v, want enumeration facet failed", err)
	}
	accepted, err := p.Validate(restricted, "p:item", declared, NeedCanonical|NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Type() != restricted || accepted.SelectedType() != BuiltinType(PrimitiveNotation) || !accepted.HasQualifiedNames() || accepted.CanonicalText() != "{urn:test}item" || accepted.IdentityKey() != "\x12\x1e{urn:test}item" {
		t.Fatalf("declared NOTATION = %#v", accepted)
	}
}

func TestProgramQualifiedResolverAdmission(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	validName := ExpandedName{Namespace: "urn:test", Local: "item"}
	for _, tc := range []struct {
		name     string
		kind     PrimitiveKind
		resolver Resolver
		want     string
	}{
		{"unresolved", PrimitiveQName, Resolver{QName: func(string) (ExpandedName, bool) { return validName, false }}, "unresolved QName"},
		{"invalid namespace UTF-8", PrimitiveQName, Resolver{QName: func(string) (ExpandedName, bool) { return ExpandedName{Namespace: "\xff", Local: "item"}, true }}, "unresolved QName"},
		{"invalid local UTF-8", PrimitiveQName, Resolver{QName: func(string) (ExpandedName, bool) { return ExpandedName{Namespace: "urn:test", Local: "\xff"}, true }}, "unresolved QName"},
		{"invalid local NCName", PrimitiveQName, Resolver{QName: func(string) (ExpandedName, bool) { return ExpandedName{Namespace: "urn:test", Local: "a:b"}, true }}, "unresolved QName"},
		{"missing declaration resolver", PrimitiveNotation, Resolver{QName: func(string) (ExpandedName, bool) { return validName, true }}, "undeclared notation"},
		{"undeclared", PrimitiveNotation, Resolver{QName: func(string) (ExpandedName, bool) { return validName, true }, Notation: func(string, string) bool { return false }}, "undeclared notation"},
		{"missing QName resolver", PrimitiveQName, Resolver{}, "unresolved QName"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, api := range qualifiedValueAPIs(p) {
				for _, needs := range []Needs{0, NeedCanonical | NeedIdentity} {
					_, err := api.validate(BuiltinType(tc.kind), "p:item", tc.resolver, needs, 16<<20)
					if err == nil || !strings.HasSuffix(err.Error(), tc.want) {
						t.Fatalf("%s needs %d resolver admission = %v, want %q", api.name, needs, err, tc.want)
					}
					if cause := errors.Unwrap(err); cause == nil || cause.Error() != tc.want {
						t.Fatalf("%s needs %d underlying cause = %v, want %q", api.name, needs, cause, tc.want)
					}
				}
			}
		})
	}
}
