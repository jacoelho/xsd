package value

import (
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

func enumerationQNameResolver() Resolver {
	return Resolver{QName: func(lexical string) (string, string, bool) {
		if lexical == "p:item" {
			return "urn:test", "item", true
		}
		return "", "", false
	}}
}

func enumerationPattern(t *testing.T, source string) *Pattern {
	t.Helper()
	pattern, err := xsdregex.Compile(source, xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return pattern
}

func TestProgramUnionEnumerationEnforcesQNameMemberFacets(t *testing.T) {
	pattern := enumerationPattern(t, "nope")
	resolver := enumerationQNameResolver()
	b := NewBuilder(BuilderOptions{})
	qname, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveQName,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	members := []TypeID{qname, builtinString}
	union, err := b.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveString, Union: members,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	restricted, err := b.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveString, Union: members,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: union, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{
			Lexical: "p:item", Type: NoType, Resolver: resolver,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.Validate(restricted, "p:item", resolver, 0, 16<<20, nil)
	if err != nil {
		t.Fatalf("QName member facet forced wrong enum value: %v", err)
	}
	if v.SelectedType() != builtinString {
		t.Fatalf("selected member = %d, want string %d", v.SelectedType(), builtinString)
	}
}

func TestProgramUnionEnumerationEnforcesListItemFacets(t *testing.T) {
	pattern := enumerationPattern(t, "nope")
	b := NewBuilder(BuilderOptions{})
	item, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := b.Add(TypeSpec{
		Variety: List, Primitive: PrimitiveString,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: item,
	})
	if err != nil {
		t.Fatal(err)
	}
	members := []TypeID{list, builtinString}
	union, err := b.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveString, Union: members,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	restricted, err := b.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveString, Union: members,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: union, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{Lexical: "x", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.Validate(restricted, "x", Resolver{}, 0, 16<<20, nil)
	if err != nil {
		t.Fatalf("list item facet forced wrong enum value: %v", err)
	}
	if v.SelectedType() != builtinString {
		t.Fatalf("selected member = %d, want string %d", v.SelectedType(), builtinString)
	}
}

func TestProgramNestedUnionEnumerationEnforcesNestedMemberFacets(t *testing.T) {
	pattern := enumerationPattern(t, "nope")
	resolver := enumerationQNameResolver()
	b := NewBuilder(BuilderOptions{})
	qname, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveQName,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	inner, err := b.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveString,
		Union:      []TypeID{qname, builtinString},
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	outer, err := b.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveString,
		Union:      []TypeID{inner},
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	restricted, err := b.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveString,
		Union:      []TypeID{inner},
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: outer, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{
			Lexical: "p:item", Type: NoType, Resolver: resolver,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Validate(restricted, "p:item", resolver, 0, 16<<20, nil); err != nil {
		t.Fatalf("nested member facet forced wrong enum value: %v", err)
	}
}

func TestProgramUnionEnumerationRetainsListMemberItems(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	item, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := b.Add(TypeSpec{
		Variety:           List,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          item,
	})
	if err != nil {
		t.Fatal(err)
	}
	union, err := b.Add(TypeSpec{
		Variety:           Union,
		Union:             []TypeID{list, builtinInt},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{
			Lexical: "1.0 2.0",
			Type:    NoType,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		bytes  bool
		needs  Needs
		input  string
		accept bool
	}{
		{name: "string value demand", input: "1 2", accept: true},
		{name: "equivalent decimal spelling", input: "1.00 2.0", accept: true},
		{name: "byte fallback", bytes: true, input: "1 2", accept: true},
		{name: "string projections", input: "1 2", needs: NeedCanonical | NeedIdentity, accept: true},
		{name: "unequal string value", input: "1 3"},
		{name: "unequal byte value", bytes: true, input: "1 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			var v Value
			if tc.bytes {
				v, err = p.ValidateBytes(union, []byte(tc.input), Resolver{}, tc.needs, 16<<20, nil)
			} else {
				v, err = p.Validate(union, tc.input, Resolver{}, tc.needs, 16<<20, nil)
			}
			if (err == nil) != tc.accept {
				t.Fatalf("validation error = %v, want accepted=%t", err, tc.accept)
			}
			if tc.accept && (v.Type() != union || v.SelectedType() != list) {
				t.Fatalf("value types = (%d, %d), want (%d, %d)", v.Type(), v.SelectedType(), union, list)
			}
		})
	}
}

func TestProgramInheritedUnionEnumerationRetainsNestedListItems(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	item, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := b.Add(TypeSpec{
		Variety:           List,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          item,
	})
	if err != nil {
		t.Fatal(err)
	}
	nested, err := b.Add(TypeSpec{
		Variety:           Union,
		Union:             []TypeID{list, builtinInt},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	baseUnion, err := b.Add(TypeSpec{
		Variety:           Union,
		Union:             []TypeID{nested, builtinString},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	restricted, err := b.Add(TypeSpec{
		Variety:           Union,
		Union:             []TypeID{nested, builtinString},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              baseUnion,
		ListItem:          NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{
			Lexical: "1.0 2.0",
			Type:    NoType,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	derived, err := b.Add(TypeSpec{
		Variety:           Union,
		Union:             []TypeID{nested, builtinString},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              restricted,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		input string
		valid bool
	}{
		{input: "1 2", valid: true},
		{input: "1.00 2.0", valid: true},
		{input: "1 3", valid: false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			_, err := p.Validate(derived, tc.input, Resolver{}, 0, 16<<20, nil)
			if (err == nil) != tc.valid {
				t.Fatalf("Validate(%q) error = %v, want valid=%t", tc.input, err, tc.valid)
			}
		})
	}
}
