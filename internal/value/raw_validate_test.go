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
	patternID, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(patternID, TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lengthID, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(lengthID, TypeSpec{
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
	id, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(id, TypeSpec{
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

func TestValidateNMTokensListItemDepth(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		maxDepth uint16
		builtin  error
		custom   error
	}{
		{name: "empty", maxDepth: 1, builtin: ErrFacet},
		{name: "XML whitespace", input: " \t\r\n", maxDepth: 1, builtin: ErrFacet},
		{name: "item depth excluded", input: "one two", maxDepth: 1, builtin: ErrLimit, custom: ErrLimit},
		{name: "depth precedes item lexical validation", input: "@", maxDepth: 1, builtin: ErrLimit, custom: ErrLimit},
		{name: "item depth admitted", input: "one two", maxDepth: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBuilder(BuilderOptions{MaxDepth: tc.maxDepth})
			custom, err := b.Reserve()
			if err != nil {
				t.Fatalf("Reserve() error = %v", err)
			}
			err = b.Complete(custom, TypeSpec{
				Variety: List, ListItem: builtinNMTOKEN, Base: NoType,
				Whitespace: WhitespaceCollapse, WhitespacePresent: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			p, err := b.Seal()
			if err != nil {
				t.Fatal(err)
			}
			for _, list := range []struct {
				name string
				id   TypeID
				want error
			}{
				{name: "builtin", id: builtinNMTOKENS, want: tc.builtin},
				{name: "custom", id: custom, want: tc.custom},
			} {
				t.Run(list.name, func(t *testing.T) {
					assertListDepthValidation(t, p, list.id, tc.input, 16<<20, listDepthResult{selected: NoType, err: list.want})
				})
			}
		})
	}
}

func TestValidateNMTokensUnionItemDepth(t *testing.T) {
	for _, tc := range []struct {
		name       string
		input      string
		maxDepth   uint16
		work       uint64
		customList bool
		wantList   bool
		want       error
	}{
		{name: "empty builtin falls through", maxDepth: 2, work: 3},
		{name: "empty custom list selected", maxDepth: 2, work: 2, customList: true, wantList: true},
		{name: "empty member depth excluded", maxDepth: 1, work: 3, want: ErrLimit},
		{name: "empty fallback budget exhausted", maxDepth: 2, work: 2, want: ErrLimit},
		{name: "item depth blocks fallback", input: "one", maxDepth: 2, work: 16 << 20, want: ErrLimit},
		{name: "item depth admitted", input: "one", maxDepth: 3, work: 16 << 20, wantList: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBuilder(BuilderOptions{MaxDepth: tc.maxDepth})
			list := builtinNMTOKENS
			if tc.customList {
				var err error
				list, err = b.Reserve()
				if err != nil {
					t.Fatalf("Reserve() error = %v", err)
				}
				err = b.Complete(list, TypeSpec{
					Variety: List, ListItem: builtinNMTOKEN, Base: NoType,
					Whitespace: WhitespaceCollapse, WhitespacePresent: true,
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			id, err := b.Reserve()
			if err != nil {
				t.Fatalf("Reserve() error = %v", err)
			}
			err = b.Complete(id, TypeSpec{
				Variety: Union, Union: []TypeID{list, builtinString},
				Base: NoType, ListItem: NoType,
				Whitespace: WhitespacePreserve, WhitespacePresent: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			p, err := b.Seal()
			if err != nil {
				t.Fatal(err)
			}
			selected := builtinString
			if tc.wantList {
				selected = list
			}
			assertListDepthValidation(t, p, id, tc.input, tc.work, listDepthResult{selected: selected, err: tc.want})
		})
	}
}

type listDepthResult struct {
	selected TypeID
	err      error
}

func assertListDepthValidation(t *testing.T, p *Program, id TypeID, input string, work uint64, want listDepthResult) {
	t.Helper()
	for _, entry := range []struct {
		name     string
		validate func() (Value, error)
	}{
		{name: "string", validate: func() (Value, error) { return p.Validate(id, input, Resolver{}, 0, work, nil) }},
		{name: "admitted", validate: func() (Value, error) { return p.ValidateAdmitted(id, input, Resolver{}, 0, work, nil) }},
		{name: "bytes", validate: func() (Value, error) { return p.ValidateBytes(id, []byte(input), Resolver{}, 0, work, nil) }},
		{name: "projected bytes", validate: func() (Value, error) {
			return p.ValidateBytes(id, []byte(input), Resolver{}, NeedCanonical, work, nil)
		}},
	} {
		t.Run(entry.name, func(t *testing.T) {
			got, err := entry.validate()
			if !errors.Is(err, want.err) {
				t.Fatalf("validation of %q error = %v, want %v", input, err, want.err)
			}
			if err == nil && (got.Type() != id || got.SelectedType() != want.selected) {
				t.Fatalf("result type = %d, selected = %d; want type %d, selected %d", got.Type(), got.SelectedType(), id, want.selected)
			}
		})
	}
}

func TestValidateBytesIntegerBoundsUseIntegerLexicalRule(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	boundedID, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(boundedID, TypeSpec{
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
	base, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(base, TypeSpec{
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
	derived, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(derived, TypeSpec{
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
	id, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(id, TypeSpec{
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
	item, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(item, TypeSpec{Variety: Atomic, Primitive: PrimitiveString, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType})
	if err != nil {
		t.Fatal(err)
	}
	list, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(list, TypeSpec{Variety: List, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: item})
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
	id, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(id, TypeSpec{
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
	id, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(id, TypeSpec{
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
		id, err := b.Reserve()
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		err = b.Complete(id, TypeSpec{
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
	id, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(id, TypeSpec{
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
		id, err := b.Reserve()
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		err = b.Complete(id, TypeSpec{
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
	id, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(id, TypeSpec{
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
	id, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(id, TypeSpec{
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
	id, err := b.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = b.Complete(id, TypeSpec{
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

type numericByteCase struct {
	input   string
	message string
}

func TestAdmittedNumericBoundNormalization(t *testing.T) {
	for _, test := range []struct {
		name, lower, upper string
		builtin            BuiltinKind
		cases              []numericByteCase
	}{
		{name: "padded-negative-integer", lower: "-00012347", upper: "-00012345", builtin: BuiltinInteger,
			cases: []numericByteCase{{input: "-12347"}, {input: "-00012346"}, {input: "-12345"},
				{input: "-12348", message: "minInclusive"}, {input: "-12344", message: "maxInclusive"}, {input: "-12346"}}},
		{name: "signed-zero-integer", lower: "-0000", upper: "+0001", builtin: BuiltinInteger,
			cases: []numericByteCase{{input: "-0000"}, {input: "+0001"}, {input: "-1", message: "minInclusive"},
				{input: "2", message: "maxInclusive"}, {input: "1.0", message: "invalid integer"}, {input: "0"}}},
		{name: "padded-decimal", lower: "0", upper: "+" + strings.Repeat("0", 1024) + "1.5000",
			cases: []numericByteCase{{input: "1.5"}, {input: "+0001.5000"}, {input: "1.5001", message: "maxInclusive"}, {input: "1.5"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			builder := NewBuilder(BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if completeErr := builder.Complete(id, TypeSpec{
				Variety: Atomic, Primitive: PrimitiveDecimal, Builtin: test.builtin,
				Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType,
				Facets: FacetSpec{
					MinInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: test.lower, Type: NoType}, Present: true},
					MaxInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: test.upper, Type: NoType}, Present: true},
				},
			}); completeErr != nil {
				t.Fatal(completeErr)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			assertNumericByteCases(t, program, id, test.cases)
		})
	}
}

func TestBorrowedNumericInheritedAndExclusiveBounds(t *testing.T) {
	builder := NewBuilder(BuilderOptions{})
	base, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec := TypeSpec{
		Variety: Atomic, Primitive: PrimitiveDecimal,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType,
		Facets: FacetSpec{
			MinInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: "1", Type: NoType}, Present: true},
			MaxInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: "10", Type: NoType}, Present: true},
		},
	}
	if completeErr := builder.Complete(base, spec); completeErr != nil {
		t.Fatal(completeErr)
	}
	derived, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec.Base = base
	spec.Facets.MinInclusive.Lexical = "2"
	spec.Facets.MaxInclusive.Lexical = "8"
	if completeErr := builder.Complete(derived, spec); completeErr != nil {
		t.Fatal(completeErr)
	}
	exclusive, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec.Base = NoType
	spec.Facets = FacetSpec{MinExclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: "1", Type: NoType}, Present: true}}
	if completeErr := builder.Complete(exclusive, spec); completeErr != nil {
		t.Fatal(completeErr)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	assertNumericByteCases(t, program, derived, []numericByteCase{
		{input: "2"}, {input: "8"}, {input: "1.5", message: "minInclusive"},
		{input: "9", message: "maxInclusive"}, {input: "2"},
	})
	assertNumericByteCases(t, program, exclusive, []numericByteCase{
		{input: "1", message: "minExclusive"}, {input: "2"},
	})
}

func assertNumericByteCases(t *testing.T, program *Program, id TypeID, cases []numericByteCase) {
	t.Helper()
	var scratch Scratch
	for _, test := range cases {
		got, err := program.ValidateBytes(id, []byte(test.input), Resolver{}, 0, 1<<20, &scratch)
		if test.message != "" {
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("ValidateBytes(%q) = %v, want %q", test.input, err, test.message)
			}
			if test.message != "invalid integer" && !errors.Is(err, ErrFacet) {
				t.Fatalf("ValidateBytes(%q) error %v lacks facet classification", test.input, err)
			}
			continue
		}
		if err != nil || got.Type() != id || got.SelectedType() != NoType || got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
			t.Fatalf("ValidateBytes(%q) = %#v, %v", test.input, got, err)
		}
	}
}

func TestPresentZeroNumericBoundAndRecovery(t *testing.T) {
	builder := NewBuilder(BuilderOptions{})
	spec := TypeSpec{
		Variety: Atomic, Primitive: PrimitiveDecimal,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	}
	unbounded, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err = builder.Complete(unbounded, spec); err != nil {
		t.Fatal(err)
	}
	bounded, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec.Facets.MaxInclusive = BoundFacet{
		Lexical: "-000.000", Type: NoType, Present: true,
	}
	if err = builder.Complete(bounded, spec); err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	assertNumericByteCases(t, program, bounded, []numericByteCase{
		{input: "-.5"}, {input: "-0"},
		{input: ".0001", message: "maxInclusive"}, {input: "0"},
	})
	assertNumericByteCases(t, program, unbounded, []numericByteCase{{input: ".0001"}})
}

func TestRawDecimalAdmittedBoundFallbacks(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, min, max string
		fraction       bool
		cases          []struct{ input, message string }
	}{
		{name: "negative", min: "-2", max: "-1", cases: []struct{ input, message string }{
			{"-1.5", ""}, {"0", "maxInclusive"},
		}},
		{name: "digits", min: "0", max: "10", fraction: true, cases: []struct{ input, message string }{
			{"1.20", ""}, {"1.25", "fractionDigits"},
		}},
		{name: "fractional", min: "+000.5000", max: "+000.5000", cases: []struct{ input, message string }{
			{".5", ""}, {".4999", "minInclusive"}, {".5001", "maxInclusive"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := NewBuilder(BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			facets := FacetSpec{
				MinInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: test.min, Type: NoType}, Present: true},
				MaxInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: test.max, Type: NoType}, Present: true},
			}
			if test.fraction {
				facets.FractionDigits = CardinalityFacet{Value: 1, Present: true}
			}
			if completeErr := builder.Complete(id, TypeSpec{
				Variety: Atomic, Primitive: PrimitiveDecimal,
				Whitespace: WhitespaceCollapse, WhitespacePresent: true,
				Base: NoType, ListItem: NoType, Facets: facets,
			}); completeErr != nil {
				t.Fatal(completeErr)
			}
			union, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if completeErr := builder.Complete(union, TypeSpec{
				Variety: Union, Union: []TypeID{id, BuiltinType(PrimitiveString)},
				Whitespace: WhitespaceCollapse, WhitespacePresent: true,
				Base: NoType, ListItem: NoType,
			}); completeErr != nil {
				t.Fatal(completeErr)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			var scratch Scratch
			for _, tc := range test.cases {
				got, err := program.ValidateBytes(id, []byte(tc.input), Resolver{}, 0, 16<<20, &scratch)
				if tc.message == "" {
					if err != nil || got.Type() != id || got.SelectedType() != NoType || got.CanonicalText() != "" || got.IdentityKey() != "" {
						t.Fatalf("ValidateBytes(%q) = %#v, %v", tc.input, got, err)
					}
				} else {
					if err == nil || !strings.Contains(err.Error(), tc.message) {
						t.Fatalf("ValidateBytes(%q) = %v, want %q", tc.input, err, tc.message)
					}
					if !errors.Is(err, ErrFacet) {
						t.Fatalf("ValidateBytes(%q) error %v lacks facet classification", tc.input, err)
					}
				}
				selected, unionErr := program.ValidateBytes(union, []byte(tc.input), Resolver{}, 0, 16<<20, &scratch)
				want := id
				if tc.message != "" {
					want = BuiltinType(PrimitiveString)
				}
				if unionErr != nil || selected.Type() != union || selected.SelectedType() != want {
					t.Fatalf("union ValidateBytes(%q) type/selection=%d/%d, error=%v; want %d/%d", tc.input, selected.Type(), selected.SelectedType(), unionErr, union, want)
				}
			}
		})
	}
}

func TestTemporalCanonicalProjectionOwnsBytes(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id            TypeID
		lexical, want string
	}{
		{builtinDateTime, "2000-02-29T24:00:00.000Z", "2000-03-01T00:00:00Z"},
		{builtinDateTime, "2000-02-29T00:30:00.1200", "2000-02-29T00:30:00.12"},
		{builtinTime, "00:30:00+01:00", "23:30:00Z"},
		{builtinTime, "12:34:56.1200Z", "12:34:56.12Z"},
		{builtinTime, "12:34:56.000Z", "12:34:56Z"},
		{builtinGMonth, "--02+00:00", "--02Z"},
		{builtinGYearMonth, "-0001-02+14:00", "-0001-02+14:00"},
		{builtinGYear, "0001", "0001"},
		{builtinGMonthDay, "--01-01+14:00", "--01-01+14:00"},
		{builtinGDay, "---01-14:00", "---01-14:00"},
		{builtinGMonth, "--12", "--12"},
	} {
		t.Run(tc.lexical, func(t *testing.T) {
			got, err := p.Validate(tc.id, tc.lexical, Resolver{}, NeedCanonical, 16<<20, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got.CanonicalText() != tc.want {
				t.Fatalf("canonical = %q, want %q", got.CanonicalText(), tc.want)
			}
			raw := []byte(tc.lexical)
			got, err = p.ValidateBytes(tc.id, raw, Resolver{}, NeedCanonical, 16<<20, nil)
			if err != nil {
				t.Fatal(err)
			}
			clear(raw)
			if got.CanonicalText() != tc.want {
				t.Fatalf("canonical after input mutation = %q, want %q", got.CanonicalText(), tc.want)
			}
		})
	}
}

func TestRawPrimitiveTokenValidationReuse(t *testing.T) {
	program, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	var scratch Scratch
	for _, tc := range []struct {
		kind    PrimitiveKind
		valid   []string
		invalid string
	}{{PrimitiveBoolean, []string{"true", "false", "1", "0"}, "trux"},
		{PrimitiveFloat, []string{"INF", "-INF", "NaN"}, "INX"},
		{PrimitiveDouble, []string{"INF", "-INF", "NaN"}, "INX"}} {
		id := BuiltinType(tc.kind)
		if _, err := program.ValidateBytes(id, []byte(tc.invalid), Resolver{}, 0, 16<<20, &scratch); err == nil {
			t.Fatalf("accepted invalid primitive %d", tc.kind)
		}
		for _, text := range tc.valid {
			got, err := program.ValidateBytes(id, []byte(text), Resolver{}, 0, 16<<20, &scratch)
			if err != nil || got.Type() != id || got.SelectedType() != NoType || got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
				t.Fatalf("primitive %d %q: type/selected/canonical %d/%d/%q, %v", tc.kind, text, got.Type(), got.SelectedType(), got.CanonicalText(), err)
			}
		}
	}
}

func TestUnprojectedStringByteResultAndWorkContract(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []struct {
		name string
		id   TypeID
	}{
		{"anySimpleType", AnySimpleTypeID}, {"string", builtinString},
	} {
		t.Run(typ.name, func(t *testing.T) {
			var scratch Scratch
			for _, test := range []struct {
				name           string
				raw            []byte
				work           uint64
				needs          Needs
				limit          bool
				canonical, key string
				identity       bool
			}{
				{name: "empty", work: 1},
				{name: "multibyte below work", raw: []byte("é"), work: 2, limit: true},
				{name: "multibyte at work", raw: []byte("é"), work: 3},
				{name: "requested projections", raw: []byte("é"), work: 3, needs: NeedCanonical | NeedIdentity, canonical: "é", key: "\x00\x1eé", identity: true},
			} {
				got, err := p.ValidateBytes(typ.id, test.raw, Resolver{}, test.needs, test.work, &scratch)
				if test.limit {
					if !errors.Is(err, ErrLimit) || got != (Value{}) {
						t.Fatalf("%s: got %+v, %v; want empty result and ErrLimit", test.name, got, err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("%s: %v", test.name, err)
				}
				if got.Type() != typ.id || got.SelectedType() != NoType {
					t.Fatalf("%s: type=%d selected=%d; want %d/%d", test.name, got.Type(), got.SelectedType(), typ.id, NoType)
				}
				if got.CanonicalText() != test.canonical || got.IdentityKey() != test.key || got.HasIdentity() != test.identity {
					t.Fatalf("%s: canonical=%q identity=%q present=%v; want %q/%q/%v", test.name, got.CanonicalText(), got.IdentityKey(), got.HasIdentity(), test.canonical, test.key, test.identity)
				}
				if got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
					t.Fatalf("%s: unexpected document or qualified-name projection", test.name)
				}
			}
		})
	}
}
