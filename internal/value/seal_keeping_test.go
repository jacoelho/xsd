package value

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

type keepingFixture struct {
	builder *Builder
	// pattern restricts xs:string by a pattern and two enumerations; no other
	// type depends on it.
	pattern TypeID
	// base restricts xs:string by maxLength 3; derived restricts base by
	// minLength 2.
	base, derived TypeID
	// item is a decimal type and list a list of item; member is a decimal
	// type and union a union of member and xs:string.
	item, list, member, union TypeID
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
	keep := make([]bool, int(BuiltinTypeCount)+len(f.builder.program.types))
	for i := range keep {
		keep[i] = !slices.Contains(dropped, TypeID(i)) // #nosec G115 -- keep covers a test builder's types.
	}
	return keep
}

func discarded(p *Program, id TypeID) bool {
	i, user := userTypeIndex(id, len(p.types))
	return user && p.types[i].variety == varietyDiscarded
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
	dropped := p.types[f.pattern-BuiltinTypeCount]
	if dropped.facets.patterns != nil || dropped.facets.enumGroups != nil || dropped.facets.present != 0 {
		t.Fatalf("unkept type retains facets: patterns %d, enumeration groups %d, present %b", len(dropped.facets.patterns), len(dropped.facets.enumGroups), dropped.facets.present)
	}
	if !discarded(p, f.pattern) {
		t.Fatal("discarded(unkept type) = false, want true")
	}
	if err := validateKeeping(p, f.pattern, "ab"); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Validate(unkept type, %q) error = %v, want ErrMetadata", "ab", err)
	}
	if _, err := p.ValidateBytes(f.pattern, []byte("ab"), Resolver{}, 0, EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil); !errors.Is(err, ErrMetadata) {
		t.Fatalf("ValidateBytes(unkept type) error = %v, want ErrMetadata", err)
	}
	for _, id := range []TypeID{f.base, f.derived, f.item, f.list, f.member, f.union, BuiltinType(PrimitiveString)} {
		if discarded(p, id) {
			t.Fatalf("discarded(%d) = true, want false", id)
		}
	}
}

func TestSealKeepingLeavesKeptTypesValidatingAsSeal(t *testing.T) {
	dropping := newKeepingFixture(t)
	keeping, err := dropping.builder.SealKeeping(dropping.keep(dropping.pattern))
	if err != nil {
		t.Fatalf("SealKeeping() error = %v", err)
	}
	f := newKeepingFixture(t)
	all, err := f.builder.Seal()
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	for _, test := range []struct {
		id      TypeID
		lexical string
		valid   bool
	}{
		{id: f.derived, lexical: "ab", valid: true},
		{id: f.derived, lexical: "a"},
		{id: f.derived, lexical: "abcd"},
		{id: f.list, lexical: "1 2.5", valid: true},
		{id: f.list, lexical: "1 x"},
		{id: f.union, lexical: "4.5", valid: true},
		{id: f.union, lexical: "abcd", valid: true},
	} {
		got, want := validateKeeping(keeping, test.id, test.lexical), validateKeeping(all, test.id, test.lexical)
		if (want == nil) != test.valid {
			t.Fatalf("Seal: Validate(%d, %q) error = %v, want valid %v", test.id, test.lexical, want, test.valid)
		}
		if (got == nil) != (want == nil) || got != nil && got.Error() != want.Error() {
			t.Fatalf("SealKeeping: Validate(%d, %q) error = %v, want %v as with every type kept", test.id, test.lexical, got, want)
		}
	}
}

func TestSealKeepingRejectsAKeptTypeWhoseDependencyIsNotKept(t *testing.T) {
	for _, test := range []struct {
		name    string
		dropped func(keepingFixture) TypeID
	}{
		{name: "base", dropped: func(f keepingFixture) TypeID { return f.base }},
		{name: "list item", dropped: func(f keepingFixture) TypeID { return f.item }},
		{name: "union member", dropped: func(f keepingFixture) TypeID { return f.member }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newKeepingFixture(t)
			dropped := test.dropped(f)
			if _, err := f.builder.SealKeeping(f.keep(dropped)); !errors.Is(err, ErrMetadata) {
				t.Fatalf("SealKeeping(without %s) error = %v, want ErrMetadata", test.name, err)
			}
			p, err := f.builder.Seal()
			if err != nil {
				t.Fatalf("Seal() after rejected SealKeeping error = %v, want the builder unchanged", err)
			}
			if discarded(p, dropped) {
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
}
