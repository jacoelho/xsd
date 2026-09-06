package value

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

func TestValidateBytesScalarFastPathsRetainNoAllocation(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	pattern, err := xsdregex.Compile("a+", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(BuilderOptions{})
	patternID, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lengthID, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Length: CardinalityFacet{Value: 64, Present: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	patternProgram, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	var scratch Scratch
	cases := []struct {
		name string
		p    *Program
		id   TypeID
		raw  []byte
		s    *Scratch
	}{
		{name: "boolean", p: p, id: builtinBoolean, raw: []byte("true")},
		{name: "integer", p: p, id: builtinInt, raw: []byte("2147483647")},
		{name: "short string", p: p, id: builtinString, raw: []byte("value")},
		{name: "long string", p: p, id: builtinString, raw: []byte("0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz")},
		{name: "pattern", p: patternProgram, id: patternID, raw: []byte("aaaa"), s: &scratch},
		{name: "unicode length", p: patternProgram, id: lengthID, raw: []byte(strings.Repeat("é", 64))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.p.ValidateBytes(tc.id, tc.raw, Resolver{}, 0, 16<<20, tc.s); err != nil {
				t.Fatal(err)
			}
			allocs := testing.AllocsPerRun(100, func() {
				if _, err := tc.p.ValidateBytes(tc.id, tc.raw, Resolver{}, 0, 16<<20, tc.s); err != nil {
					t.Fatal(err)
				}
			})
			if allocs != 0 {
				t.Fatalf("ValidateBytes allocations = %v, want 0", allocs)
			}
		})
	}
}

func TestValidateBytesFallsBackForProjectionAndList(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.ValidateBytes(builtinString, []byte("value"), Resolver{}, NeedCanonical, 16<<20, nil); err != nil {
		t.Fatal(err)
	}

	b := NewBuilder(BuilderOptions{})
	item, err := b.Add(TypeSpec{Variety: Atomic, Primitive: PrimitiveString, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType})
	if err != nil {
		t.Fatal(err)
	}
	list, err := b.Add(TypeSpec{Variety: List, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: item})
	if err != nil {
		t.Fatal(err)
	}
	p, err = b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.ValidateBytes(list, []byte("one two"), Resolver{}, NeedCanonical, 16<<20, nil)
	if err != nil || v.CanonicalText() != "one two" {
		t.Fatalf("list raw validation = %q, %v", v.CanonicalText(), err)
	}
}

func TestValidateBytesDecimalInclusiveBoundsAvoidStringMaterialization(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{
			MinInclusive: BoundFacet{
				LiteralSpec: LiteralSpec{Lexical: "0", Type: NoType},
				Present:     true,
			},
			MaxInclusive: BoundFacet{
				LiteralSpec: LiteralSpec{Lexical: "999999", Type: NoType},
				Present:     true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, lexical := range []string{"42.50", "0", "999999", "-0.0", " 42.50 ", "\t0\n", " -0.0 "} {
		if _, err := p.ValidateBytes(id, []byte(lexical), Resolver{}, 0, 16<<20, nil); err != nil {
			t.Fatalf("ValidateBytes(%q) error = %v", lexical, err)
		}
	}
	if _, err := p.ValidateBytes(id, []byte("1000000"), Resolver{}, 0, 16<<20, nil); err == nil {
		t.Fatal("ValidateBytes accepted a value above maxInclusive")
	}
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := p.ValidateBytes(id, []byte("42.50"), Resolver{}, 0, 16<<20, nil); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("bounded decimal ValidateBytes allocations = %v, want 0", allocs)
	}
}

func TestValidateBytesUnionPreservesSelectionAndProjections(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety: Union, Union: []TypeID{builtinBoolean, builtinID, builtinString},
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		lexical  string
		selected TypeID
		ids      string
	}{
		{lexical: "true", selected: builtinBoolean},
		{lexical: "item", selected: builtinID, ids: "item"},
		{lexical: "two words", selected: builtinString},
		{lexical: "\t item  ", selected: builtinID, ids: "item"},
	} {
		t.Run(tc.lexical, func(t *testing.T) {
			raw := []byte(tc.lexical)
			v, err := p.ValidateBytes(id, raw, Resolver{}, 0, 16<<20, nil)
			if err != nil {
				t.Fatal(err)
			}
			clear(raw)
			if v.Type() != id || v.SelectedType() != tc.selected || v.IDs() != tc.ids {
				t.Fatalf("union result = type %d, selected %d, IDs %q; want %d, %d, %q", v.Type(), v.SelectedType(), v.IDs(), id, tc.selected, tc.ids)
			}
		})
	}
}

func TestValidateBytesUnionChargesFailedAttempts(t *testing.T) {
	for _, budget := range []uint64{4, 6} {
		b := NewBuilder(BuilderOptions{})
		id, err := b.Add(TypeSpec{
			Variety: Union, Union: []TypeID{builtinBoolean, builtinString},
			Whitespace: WhitespaceCollapse, WhitespacePresent: true,
			Base: NoType, ListItem: NoType,
		})
		if err != nil {
			t.Fatal(err)
		}
		p, err := b.Seal()
		if err != nil {
			t.Fatal(err)
		}
		_, err = p.ValidateBytes(id, []byte("x"), Resolver{}, 0, budget, nil)
		if budget == 4 && !errors.Is(err, ErrLimit) || budget == 6 && err != nil {
			t.Fatalf("work budget %d: %v", budget, err)
		}
	}
}

func TestValidateBytesFallbackRetainsRawWork(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety: Union, Union: []TypeID{builtinBoolean, builtinQName},
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateBytes(id, []byte("x"), Resolver{}, 0, 6, nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("fallback work was not retained: %v", err)
	}
}

func TestValidateBytesTextFacetsCountNormalizedCharacters(t *testing.T) {
	pattern, err := xsdregex.Compile("é.", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, whitespace := range []WhitespaceMode{WhitespacePreserve, WhitespaceCollapse} {
		b := NewBuilder(BuilderOptions{})
		id, err := b.Add(TypeSpec{
			Variety: Atomic, Primitive: PrimitiveString,
			Whitespace: whitespace, WhitespacePresent: true,
			Base: NoType, ListItem: NoType,
			Facets: FacetSpec{
				Length:   CardinalityFacet{Value: 2, Present: true},
				Patterns: [][]*Pattern{{pattern}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		p, err := b.Seal()
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			lexical string
			valid   bool
		}{
			{lexical: "éx", valid: true},
			{lexical: "é", valid: false},
			{lexical: "éxy", valid: false},
			{lexical: "xx", valid: false},
			{lexical: "  éx ", valid: whitespace == WhitespaceCollapse},
		} {
			_, err := p.ValidateBytes(id, []byte(tc.lexical), Resolver{}, 0, 16<<20, nil)
			if (err == nil) != tc.valid {
				t.Fatalf("whitespace %d, value %q: %v, want valid %v", whitespace, tc.lexical, err, tc.valid)
			}
		}
	}
}

func TestValidateBytesChecksCustomIntegerLexicalSpace(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveDecimal, Builtin: BuiltinInteger,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, lexical := range []string{"1.0", "1.5", ".0"} {
		if _, err := p.ValidateBytes(id, []byte(lexical), Resolver{}, 0, 16<<20, nil); err == nil {
			t.Fatalf("integer accepted %q", lexical)
		}
	}
	if _, err := p.ValidateBytes(id, []byte("123"), Resolver{}, 0, 16<<20, nil); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBytesUnionRetainsExplicitIdentity(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety: Union, Union: []TypeID{builtinID}, Identity: IdentityID,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.ValidateBytes(id, []byte("item"), Resolver{}, 0, 16<<20, nil)
	if err != nil || v.IDs() != "item" {
		t.Fatalf("union identity = %q, %v", v.IDs(), err)
	}
}
