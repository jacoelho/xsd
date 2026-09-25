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

func TestValidateAdmittedSkipsXMLAdmissionForOwnedString(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	lexical := "accepted\x01owned"
	_, err = p.Validate(builtinString, lexical, Resolver{}, NeedCanonical, 16<<20, nil)
	if err == nil {
		t.Fatal("Validate accepted a non-XML character")
	}
	got, err := p.ValidateAdmitted(builtinString, lexical, Resolver{}, NeedCanonical, uint64(len(lexical))+1, nil)
	if err != nil {
		t.Fatalf("ValidateAdmitted error = %v", err)
	}
	if got.CanonicalText() != lexical {
		t.Fatalf("ValidateAdmitted canonical = %q, want %q", got.CanonicalText(), lexical)
	}
	if _, err := p.ValidateAdmitted(builtinString, lexical, Resolver{}, 0, uint64(len(lexical)), nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("ValidateAdmitted budget error = %v, want ErrLimit", err)
	}
}

func TestValidateBytesStringEnumerationRetainsBorrowedInput(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{Lexical: "ready"}, {Lexical: ""}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		raw   []byte
		valid bool
	}{
		{raw: []byte("ready"), valid: true},
		{raw: []byte(""), valid: true},
		{raw: []byte(" ready"), valid: false},
		{raw: []byte("other"), valid: false},
	} {
		raw := append([]byte(nil), tc.raw...)
		got, err := p.ValidateBytes(id, raw, Resolver{}, 0, 16<<20, nil)
		if (err == nil) != tc.valid {
			t.Fatalf("ValidateBytes(%q) error = %v, valid = %v", raw, err, tc.valid)
		}
		clear(raw)
		if tc.valid && (got.Type() != id || got.CanonicalText() != "") {
			t.Fatalf("unprojected enum result = %#v, want type %d and empty canonical", got, id)
		}
	}
	raw := []byte("ready")
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := p.ValidateBytes(id, raw, Resolver{}, 0, 16<<20, nil); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("string enumeration allocations = %v, want 0", allocs)
	}
}

func TestValidateBytesNMTokensListChargesEveryItem(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	id, ok := BuiltinTypeID("NMTOKENS")
	if !ok {
		t.Fatal("missing NMTOKENS builtin")
	}
	for _, tc := range []struct {
		name     string
		input    string
		limit    uint64
		valid    bool
		limitErr bool
	}{
		{name: "valid", input: "one  two\tthree", limit: uint64(1 + len("one  two\tthree") + (1 + len("one")) + (1 + len("two")) + (1 + len("three"))), valid: true},
		{name: "budget boundary", input: "one two", limit: uint64(1 + len("one two") + (1 + len("one")) + (1 + len("two")) - 1), valid: false, limitErr: true},
		{name: "unicode", input: "café", limit: uint64(1 + len("café") + (1 + len("café"))), valid: true},
		{name: "empty", input: "   ", limit: 4, valid: false},
		{name: "invalid item", input: "one two@", limit: uint64(1 + len("one two@") + (1 + len("one")) + (1 + len("two@"))), valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(tc.input)
			got, err := p.ValidateBytes(id, raw, Resolver{}, 0, tc.limit, nil)
			if (err == nil) != tc.valid {
				t.Fatalf("ValidateBytes(%q) error = %v, valid = %v", tc.input, err, tc.valid)
			}
			if tc.limitErr && !errors.Is(err, ErrLimit) {
				t.Fatalf("ValidateBytes(%q) error = %v, want ErrLimit", tc.input, err)
			}
			if tc.valid && got.Type() != id {
				t.Fatalf("result type = %d, want %d", got.Type(), id)
			}
		})
	}
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := p.ValidateBytes(id, []byte("one two three"), Resolver{}, 0, 64, nil); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("NMTOKENS allocations = %v, want 0", allocs)
	}
}

func TestValidateBytesNMTokensHonorsItemDepth(t *testing.T) {
	id, ok := BuiltinTypeID("NMTOKENS")
	if !ok {
		t.Fatal("missing NMTOKENS builtin")
	}
	for _, tc := range []struct {
		name     string
		maxDepth uint16
		valid    bool
	}{
		{name: "item depth excluded", maxDepth: 1, valid: false},
		{name: "item depth admitted", maxDepth: 2, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewBuilder(BuilderOptions{MaxDepth: tc.maxDepth}).Seal()
			if err != nil {
				t.Fatal(err)
			}
			_, err = p.ValidateBytes(id, []byte("one two"), Resolver{}, 0, 16<<20, nil)
			if (err == nil) != tc.valid {
				t.Fatalf("ValidateBytes depth %d error = %v, valid = %v", tc.maxDepth, err, tc.valid)
			}
			if !tc.valid && !errors.Is(err, ErrLimit) {
				t.Fatalf("ValidateBytes depth %d error = %v, want ErrLimit", tc.maxDepth, err)
			}
		})
	}
}

func TestValidateBytesIntegerBoundsUseIntegerLexicalRule(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	boundedID, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveDecimal, Builtin: BuiltinInteger,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{
			MinInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: "-100", Type: NoType}, Present: true},
			MaxInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: "100", Type: NoType}, Present: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		id    TypeID
		input string
		valid bool
	}{
		{name: "integer whitespace", id: builtinInteger, input: "  +001  ", valid: true},
		{name: "integer fraction rejected", id: builtinInteger, input: "1.0", valid: false},
		{name: "integer zero", id: builtinInteger, input: "0", valid: true},
		{name: "integer positive zero", id: builtinInteger, input: "+0", valid: true},
		{name: "integer negative zero", id: builtinInteger, input: "-0", valid: true},
		{name: "integer huge unbounded", id: builtinInteger, input: strings.Repeat("9", 256), valid: true},
		{name: "non-negative zero", id: builtinNonNegativeInteger, input: "0", valid: true},
		{name: "non-negative positive zero", id: builtinNonNegativeInteger, input: "+0", valid: true},
		{name: "non-negative negative zero", id: builtinNonNegativeInteger, input: "-0", valid: true},
		{name: "non-negative negative value", id: builtinNonNegativeInteger, input: "-1", valid: false},
		{name: "positive zero", id: builtinPositiveInteger, input: "0", valid: false},
		{name: "positive leading zero", id: builtinPositiveInteger, input: "+001", valid: true},
		{name: "int upper boundary", id: builtinInt, input: "2147483647", valid: true},
		{name: "int above upper boundary", id: builtinInt, input: "2147483648", valid: false},
		{name: "int lower boundary", id: builtinInt, input: "-2147483648", valid: true},
		{name: "int below lower boundary", id: builtinInt, input: "-2147483649", valid: false},
		{name: "unsigned huge boundary", id: builtinUnsignedLong, input: "18446744073709551615", valid: true},
		{name: "unsigned huge overflow", id: builtinUnsignedLong, input: "18446744073709551616", valid: false},
		{name: "custom bound lower", id: boundedID, input: "-100", valid: true},
		{name: "custom bound upper", id: boundedID, input: "+00100", valid: true},
		{name: "custom bound overflow", id: boundedID, input: "101", valid: false},
		{name: "custom bound fraction", id: boundedID, input: "1.0", valid: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.ValidateBytes(tc.id, []byte(tc.input), Resolver{}, 0, 16<<20, nil)
			if (err == nil) != tc.valid {
				t.Fatalf("ValidateBytes(%q) error = %v, valid = %v", tc.input, err, tc.valid)
			}
		})
	}
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := p.ValidateBytes(builtinInt, []byte("2147483647"), Resolver{}, 0, 64, nil); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("integer allocations = %v, want 0", allocs)
	}
}

func TestValidateBytesInheritedStringEnumerationUsesEveryGroup(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	base, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{
			{Lexical: "a", Type: NoType}, {Lexical: "b", Type: NoType},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	derived, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: base, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{Lexical: "b", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateBytes(derived, []byte("b"), Resolver{}, 0, 16<<20, nil); err != nil {
		t.Fatalf("inherited enumeration rejected b: %v", err)
	}
	if _, err := p.ValidateBytes(derived, []byte("a"), Resolver{}, 0, 16<<20, nil); err == nil {
		t.Fatal("inherited enumeration accepted a")
	}
}

func TestValidateBytesDecimalFractionDigitsZeroUsesDecimalValueSpace(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveDecimal,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{FractionDigits: CardinalityFacet{Value: 0, Present: true}},
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
		{lexical: "1.0", valid: true},
		{lexical: "1.00", valid: true},
		{lexical: "1.1", valid: false},
	} {
		_, err := p.ValidateBytes(id, []byte(tc.lexical), Resolver{}, 0, 16<<20, nil)
		if (err == nil) != tc.valid {
			t.Fatalf("ValidateBytes(%q) error = %v, valid = %v", tc.lexical, err, tc.valid)
		}
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

func TestValidateBytesUnionRetainsIDREFSelectionAndProjection(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety: Union, Union: []TypeID{builtinIDREF, builtinString},
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
	v, err := p.ValidateBytes(id, []byte("ref"), Resolver{}, NeedCanonical|NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type() != id || v.SelectedType() != builtinIDREF || v.IDRefs() != "ref" {
		t.Fatalf("union value = type %d, selected %d, IDRefs %q; want type %d, selected %d, IDRefs %q", v.Type(), v.SelectedType(), v.IDRefs(), id, builtinIDREF, "ref")
	}
}
