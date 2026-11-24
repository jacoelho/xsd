package value

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

type keepingFixture struct {
	builder                                           *Builder
	pattern, base, derived, item, list, member, union TypeID
}

func newKeepingFixture(t *testing.T) keepingFixture {
	t.Helper()
	pattern, err := xsdregex.Compile("[a-z]+")
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	str := BuiltinType(PrimitiveString)
	add := func(spec TypeSpec) TypeID {
		t.Helper()
		id, err := b.Reserve()
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		if err := b.Complete(id, spec); err != nil {
			t.Fatalf("Complete(%d) error = %v", id, err)
		}
		return id
	}
	f := keepingFixture{builder: b}
	f.pattern = add(TypeSpec{Variety: Atomic, Primitive: PrimitiveString, Base: str, ListItem: NoType, Facets: FacetSpec{
		Patterns:    [][]*Pattern{{pattern}},
		Enumeration: []LiteralSpec{{Lexical: "ab", Type: NoType}, {Lexical: "cd", Type: NoType}},
	}})
	f.base = add(TypeSpec{Variety: Atomic, Primitive: PrimitiveString, Base: str, ListItem: NoType, Facets: FacetSpec{
		MaxLength: CardinalityFacet{Value: 3, Present: true},
	}})
	f.derived = add(TypeSpec{Variety: Atomic, Primitive: PrimitiveString, Base: f.base, ListItem: NoType, Facets: FacetSpec{
		MinLength: CardinalityFacet{Value: 2, Present: true},
	}})
	f.item = add(TypeSpec{Variety: Atomic, Primitive: PrimitiveDecimal, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType})
	f.list = add(TypeSpec{Variety: List, ListItem: f.item, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType})
	f.member = add(TypeSpec{Variety: Atomic, Primitive: PrimitiveDecimal, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType})
	f.union = add(TypeSpec{Variety: Union, Union: []TypeID{f.member, str}, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType})
	return f
}

// keep marks every type kept except dropped.
func (f keepingFixture) keep(dropped ...TypeID) []bool {
	keep := make([]bool, f.builder.UserTypeCount())
	for i := range keep {
		keep[i] = !slices.Contains(dropped, BuiltinTypeCount+TypeID(i)) // #nosec G115 -- keep covers a test builder's user types.
	}
	return keep
}

// discarded reports whether p rejects id as metadata it does not hold. lexical
// is valid for id when its definition is kept.
func discarded(t *testing.T, p *Program, id TypeID, lexical string) bool {
	t.Helper()
	_, hasInputs := p.InputRequirements(id)
	err := validateKeeping(p, id, lexical)
	if hasInputs == errors.Is(err, ErrMetadata) {
		t.Fatalf("type %d: InputRequirements ok %v, Validate(%q) error %v; want both to agree on whether it is kept", id, hasInputs, lexical, err)
	}
	return !hasInputs
}

func validateKeeping(p *Program, id TypeID, lexical string) error {
	_, err := p.Validate(id, lexical, Resolver{}, NeedCanonical, EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
	return err
}

func TestSealKeepingDropsTheDefinitionOfAnUnkeptType(t *testing.T) {
	f := newKeepingFixture(t)
	p, err := f.builder.SealKeeping(f.keep(f.pattern))
	if err != nil {
		t.Fatalf("SealKeeping() error = %v", err)
	}
	if !discarded(t, p, f.pattern, "ab") {
		t.Fatal("unkept type is kept, want discarded")
	}
	if _, err := p.ValidateBytes(f.pattern, []byte("ab"), Resolver{}, 0, EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil); !errors.Is(err, ErrMetadata) {
		t.Fatalf("ValidateBytes(unkept type) error = %v, want ErrMetadata", err)
	}
	for _, kept := range []struct {
		id      TypeID
		lexical string
	}{
		{id: f.base, lexical: "ab"},
		{id: f.derived, lexical: "ab"},
		{id: f.item, lexical: "1"},
		{id: f.list, lexical: "1 2"},
		{id: f.member, lexical: "1"},
		{id: f.union, lexical: "1"},
		{id: BuiltinType(PrimitiveString), lexical: "ab"},
	} {
		if discarded(t, p, kept.id, kept.lexical) {
			t.Fatalf("kept type %d is discarded", kept.id)
		}
	}
}

func TestUnkeptTypeFailsWithErrMetadataBeforeItsTypeLookupCanFail(t *testing.T) {
	f := newKeepingFixture(t)
	p, err := f.builder.SealKeeping(f.keep(f.pattern))
	if err != nil {
		t.Fatalf("SealKeeping() error = %v", err)
	}
	generous := EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}
	exhausted := EvaluationLimits{Work: 1, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}
	for _, test := range []struct {
		name     string
		validate func(TypeID) error
	}{
		{name: "invalid XML character", validate: func(id TypeID) error {
			_, err := p.Validate(id, "a\x01b", Resolver{}, NeedCanonical, generous, nil)
			return err
		}},
		{name: "exhausted work", validate: func(id TypeID) error {
			_, err := p.Validate(id, "ab", Resolver{}, NeedCanonical, exhausted, nil)
			return err
		}},
		{name: "exhausted work on bytes", validate: func(id TypeID) error {
			_, err := p.ValidateBytes(id, []byte("ab"), Resolver{}, 0, exhausted, nil)
			return err
		}},
	} {
		if err := test.validate(f.base); err == nil || errors.Is(err, ErrMetadata) {
			t.Fatalf("%s: Validate(kept type) error = %v, want a failure other than ErrMetadata", test.name, err)
		}
		if err := test.validate(f.pattern); !errors.Is(err, ErrMetadata) || errors.Is(err, ErrLimit) {
			t.Fatalf("%s: Validate(unkept type) error = %v, want ErrMetadata alone", test.name, err)
		}
	}
}

func TestSealKeepingLeavesKeptTypesValidatingAsKeepingEveryType(t *testing.T) {
	dropping := newKeepingFixture(t)
	keeping, err := dropping.builder.SealKeeping(dropping.keep(dropping.pattern))
	if err != nil {
		t.Fatalf("SealKeeping() error = %v", err)
	}
	f := newKeepingFixture(t)
	all, err := sealAll(f.builder)
	if err != nil {
		t.Fatalf("sealAll() error = %v", err)
	}
	derived := func(f keepingFixture) TypeID { return f.derived }
	list := func(f keepingFixture) TypeID { return f.list }
	union := func(f keepingFixture) TypeID { return f.union }
	for _, test := range []struct {
		id      func(keepingFixture) TypeID
		lexical string
		valid   bool
	}{
		{id: derived, lexical: "ab", valid: true},
		{id: derived, lexical: "a"},
		{id: derived, lexical: "abcd"},
		{id: list, lexical: "1 2.5", valid: true},
		{id: list, lexical: "1 x"},
		{id: union, lexical: "4.5", valid: true},
		{id: union, lexical: "abcd", valid: true},
	} {
		got := validateKeeping(keeping, test.id(dropping), test.lexical)
		want := validateKeeping(all, test.id(f), test.lexical)
		if (want == nil) != test.valid {
			t.Fatalf("sealAll: Validate(%d, %q) error = %v, want valid %v", test.id(f), test.lexical, want, test.valid)
		}
		if (got == nil) != (want == nil) || got != nil && got.Error() != want.Error() {
			t.Fatalf("SealKeeping: Validate(%d, %q) error = %v, want %v as with every type kept", test.id(dropping), test.lexical, got, want)
		}
	}
}

func TestSealKeepingRejectsAKeptTypeWhoseDependencyIsNotKept(t *testing.T) {
	for _, test := range []struct {
		name    string
		dropped func(keepingFixture) TypeID
		lexical string
	}{
		{name: "base", dropped: func(f keepingFixture) TypeID { return f.base }, lexical: "ab"},
		{name: "list item", dropped: func(f keepingFixture) TypeID { return f.item }, lexical: "1"},
		{name: "union member", dropped: func(f keepingFixture) TypeID { return f.member }, lexical: "1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newKeepingFixture(t)
			dropped := test.dropped(f)
			if _, err := f.builder.SealKeeping(f.keep(dropped)); !errors.Is(err, ErrMetadata) {
				t.Fatalf("SealKeeping(without %s) error = %v, want ErrMetadata", test.name, err)
			}
			p, err := sealAll(f.builder)
			if err != nil {
				t.Fatalf("sealAll() after rejected SealKeeping error = %v, want the builder unchanged", err)
			}
			if discarded(t, p, dropped, test.lexical) {
				t.Fatalf("discarded(%s) = true after rejected SealKeeping", test.name)
			}
		})
	}
}

func TestSealKeepingRejectsAKeepThatDoesNotCoverEveryType(t *testing.T) {
	f := newKeepingFixture(t)
	keep := f.keep()
	if _, err := f.builder.SealKeeping(keep[:len(keep)-1]); !errors.Is(err, ErrMetadata) {
		t.Fatalf("SealKeeping(short keep) error = %v, want ErrMetadata", err)
	}
	if _, err := f.builder.SealKeeping(append(keep, true)); !errors.Is(err, ErrMetadata) {
		t.Fatalf("SealKeeping(long keep) error = %v, want ErrMetadata", err)
	}
	if _, err := f.builder.SealKeeping(nil); !errors.Is(err, ErrMetadata) {
		t.Fatalf("SealKeeping(nil) error = %v, want ErrMetadata", err)
	}
	if _, err := sealAll(f.builder); err != nil {
		t.Fatalf("sealAll() after rejected keeps error = %v, want the builder unchanged", err)
	}
}
