package value

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

func TestProgramReservesBuiltinsAndEvaluatesDecimalFacets(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{
			MinInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: "1.5", Type: NoType}, Present: true},
			MaxExclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: "4", Type: NoType}, Present: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != BuiltinTypeCount {
		t.Fatalf("first user type = %d, want %d", id, BuiltinTypeCount)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.Validate(id, " 2.00 ", Resolver{}, NeedCanonical|NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.CanonicalText() != "2.0" || v.IdentityKey() == "" {
		t.Fatalf("unexpected projections: canonical=%q identity=%q", v.CanonicalText(), v.IdentityKey())
	}
	if _, err := p.Validate(id, "4", Resolver{}, NeedCanonical, nil); err == nil {
		t.Fatal("exclusive upper facet should reject boundary")
	}
}

func TestProgramListUnionAndPatternGroups(t *testing.T) {
	pattern, err := xsdregex.Compile(".*2.*", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
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
		ListItem:          item,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Identity:          IdentityIDREFList,
		Base:              NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	union, err := b.Add(TypeSpec{
		Variety:           Union,
		Union:             []TypeID{item, BuiltinType(PrimitiveString)},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets:            FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.Validate(list, " 1.0\t2.0 ", Resolver{}, NeedCanonical|NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.CanonicalText() != "1.0 2.0" || v.IDRefs() != "1.0 2.0" {
		t.Fatalf("unexpected list projections: canonical=%q refs=%q", v.CanonicalText(), v.IDRefs())
	}
	v, err = p.Validate(union, "2", Resolver{}, NeedCanonical, nil)
	if err != nil || v.SelectedType() != item {
		t.Fatalf("union selection = %d, err=%v; want %d", v.SelectedType(), err, item)
	}
	if _, err := p.Validate(union, "no-two", Resolver{}, NeedCanonical, nil); err == nil {
		t.Fatal("pattern group should reject a nonmatching union value")
	}
}

func TestProgramListEnumerationRetainsOnlyFacetComparisonState(t *testing.T) {
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
	if _, err := p.Validate(list, "1 2", Resolver{}, 0, nil); err != nil {
		t.Fatalf("list enumeration rejected equal value-space spelling: %v", err)
	}
	if _, err := p.Validate(list, "1 3", Resolver{}, 0, nil); err == nil {
		t.Fatal("list enumeration accepted a different value")
	}
}

func TestProgramListDoesNotTreatUnicodeBytesAsWhitespace(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	item, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
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
		Facets:            FacetSpec{Length: CardinalityFacet{Value: 1, Present: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Validate(list, "a\u0120b", Resolver{}, 0, nil); err != nil {
		t.Fatalf("Unicode list item was split as whitespace: %v", err)
	}
}

func TestProgramDoesNotMaterializeUnrequestedProjections(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
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
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.Validate(id, "2.00", Resolver{}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.CanonicalText() != "" || v.IdentityKey() != "" || v.IDs() != "" || v.IDRefs() != "" {
		t.Fatalf("unrequested projections were retained: %+v", v)
	}
}

func TestProgramRejectsInvalidUTF8AndXMLCharacters(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Validate(id, string([]byte{0xff}), Resolver{}, 0, nil); err == nil {
		t.Fatal("invalid UTF-8 was accepted")
	}
	if _, err := p.Validate(id, "a\x00b", Resolver{}, 0, nil); err == nil {
		t.Fatal("invalid XML character was accepted")
	}
}

func TestProgramEqualRequiresExplicitIdentityProjection(t *testing.T) {
	stringBuilder := NewBuilder(BuilderOptions{})
	stringID, err := stringBuilder.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	decimalID, err := stringBuilder.Add(TypeSpec{
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
	p, err := stringBuilder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	stringValue, err := p.Validate(stringID, "1", Resolver{}, NeedCanonical|NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	decimalValue, err := p.Validate(decimalID, "1", Resolver{}, NeedCanonical|NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stringValue.Equal(decimalValue) {
		t.Fatal("string and decimal values with equal canonical text compared equal")
	}
	empty, err := p.Validate(stringID, "", Resolver{}, NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	otherEmpty, err := p.Validate(stringID, "", Resolver{}, NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !empty.Equal(otherEmpty) {
		t.Fatal("empty string did not compare equal with identity projection")
	}
}

func TestProgramEqualityDistinguishesEmptyListFromEmptyString(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	item, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
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
	stringType, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	left, err := p.Validate(list, "", Resolver{}, NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	right, err := p.Validate(stringType, "", Resolver{}, NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if left.IdentityKey() != right.IdentityKey() {
		t.Fatalf("test setup identity keys differ: %q and %q", left.IdentityKey(), right.IdentityKey())
	}
	if left.Equal(right) {
		t.Fatal("empty list compared equal to empty string")
	}
}

func TestProgramCompilesForwardBaseFacetsBeforeDerived(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	baseID := BuiltinTypeCount + 1
	derivedID, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              baseID,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	if derivedID != BuiltinTypeCount {
		t.Fatalf("derived ID = %d, want %d", derivedID, BuiltinTypeCount)
	}
	if _, err = b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{
			MinInclusive: BoundFacet{LiteralSpec: LiteralSpec{Lexical: "10", Type: NoType}, Present: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Validate(derivedID, "9", Resolver{}, 0, nil); err == nil {
		t.Fatal("derived type accepted value below forward base bound")
	}
}

func TestProgramDepthLimitIsExplicit(t *testing.T) {
	b := NewBuilder(BuilderOptions{MaxDepth: 1})
	future := BuiltinTypeCount + 1
	if _, err := b.Add(TypeSpec{
		Variety:           Union,
		Base:              NoType,
		ListItem:          NoType,
		Union:             []TypeID{future},
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Seal(); !errors.Is(err, ErrLimit) {
		t.Fatalf("Seal error = %v, want ErrLimit", err)
	}
	b = NewBuilder(BuilderOptions{MaxDepth: 1})
	id, err := b.Add(TypeSpec{
		Variety:           Union,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Union:             []TypeID{BuiltinType(PrimitiveString)},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Validate(id, "x", Resolver{}, 0, nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("Validate error = %v, want ErrLimit", err)
	}
}

func TestProgramEvaluationWorkLimitCountsUnionAttempts(t *testing.T) {
	b := NewBuilder(BuilderOptions{MaxEvalWork: 4})
	id, err := b.Add(TypeSpec{
		Variety:           Union,
		Union:             []TypeID{BuiltinType(PrimitiveDecimal), BuiltinType(PrimitiveString)},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Validate(id, "x", Resolver{}, 0, nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("Validate over union work budget = %v, want ErrLimit", err)
	}
}

func TestProgramBoundsFacetDependencyDepthAndStorage(t *testing.T) {
	if _, err := NewBuilder(BuilderOptions{MaxTypes: uint32(BuiltinTypeCount) - 1}).Seal(); !errors.Is(err, ErrLimit) {
		t.Fatalf("Seal below builtin type budget = %v, want ErrLimit", err)
	}

	b := NewBuilder(BuilderOptions{MaxDepth: 1})
	derived, err := b.Add(TypeSpec{
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
	if derived != BuiltinTypeCount {
		t.Fatalf("derived id = %d, want %d", derived, BuiltinTypeCount)
	}
	base := BuiltinTypeCount + 1
	if _, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{MinInclusive: BoundFacet{
			Present:     true,
			LiteralSpec: LiteralSpec{Lexical: "1", Type: base},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Seal(); !errors.Is(err, ErrLimit) {
		t.Fatalf("Seal facet dependency over depth budget = %v, want ErrLimit", err)
	}

	emptyLiterals := make([]LiteralSpec, 64)
	b = NewBuilder(BuilderOptions{MaxStorageBytes: 512 + 32*uint64(len(emptyLiterals)) - 1})
	if _, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets:            FacetSpec{Enumeration: emptyLiterals},
	}); !errors.Is(err, ErrLimit) {
		t.Fatalf("Add oversized empty enumeration = %v, want ErrLimit", err)
	}
}

func TestProgramRejectsMissingUnionMember(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	if _, err := b.Add(TypeSpec{
		Variety:           Union,
		Union:             []TypeID{NoType},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Seal(); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Seal missing union member = %v, want ErrMetadata", err)
	}
}

func TestProgramQNameLengthFacetIsAlwaysSatisfied(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveQName,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets:            FacetSpec{Length: CardinalityFacet{Value: 1, Present: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	resolve := Resolver{QName: func(string) (string, string, bool) { return "urn:test", "long-local", true }}
	if _, err := p.Validate(id, "p:long-local", resolve, 0, nil); err != nil {
		t.Fatalf("QName length facet rejected valid QName: %v", err)
	}

	b = NewBuilder(BuilderOptions{})
	if _, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets:            FacetSpec{Length: CardinalityFacet{Value: 1, Present: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Seal(); err == nil {
		t.Fatal("decimal length facet was accepted")
	}
}

func TestProgramAcceptsCompiledXSDRegexPattern(t *testing.T) {
	pattern, err := xsdregex.Compile("a+", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(BuilderOptions{})
	id, err := b.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{
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
	if _, err := p.Validate(id, "aaa", Resolver{}, NeedCanonical, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Validate(id, "b", Resolver{}, NeedCanonical, nil); err == nil {
		t.Fatal("compiled XSD pattern should reject b")
	}
}

func TestProgramQNameAndNotationResolver(t *testing.T) {
	resolve := Resolver{
		QName: func(s string) (string, string, bool) {
			if s == "p:item" {
				return "urn:test", "item", true
			}
			return "", "", false
		},
		Notation: func(ns, local string) bool { return ns == "urn:test" && local == "item" },
	}
	b := NewBuilder(BuilderOptions{})
	qname, err := b.Add(TypeSpec{Variety: Atomic, Primitive: PrimitiveQName, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType})
	if err != nil {
		t.Fatal(err)
	}
	notation, err := b.Add(TypeSpec{Variety: Atomic, Primitive: PrimitiveNotation, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	q, err := p.Validate(qname, "p:item", resolve, NeedCanonical, nil)
	if err != nil || q.CanonicalText() != "{urn:test}item" {
		t.Fatalf("QName = %q, err=%v", q.CanonicalText(), err)
	}
	n, err := p.Validate(notation, "p:item", resolve, NeedCanonical, nil)
	if err != nil || n.CanonicalText() != "{urn:test}item" {
		t.Fatalf("NOTATION = %q, err=%v", n.CanonicalText(), err)
	}
}

func TestProgramNotationOnlyResolverValidatesAndStoresExpandedName(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	notation, err := b.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveNotation,
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

	var resolved []string
	resolve := Resolver{
		Notation: func(namespace, local string) bool {
			resolved = append(resolved, namespace+"/"+local)
			return namespace == "" && (local == "item" || local == "other")
		},
	}
	item, err := p.Validate(notation, "item", resolve, NeedCanonical|NeedIdentity, nil)
	if err != nil {
		t.Fatalf("declared unqualified NOTATION = %v", err)
	}
	if item.CanonicalText() != "item" || item.IdentityKey() == "" {
		t.Fatalf("declared NOTATION projections = canonical %q, identity %q", item.CanonicalText(), item.IdentityKey())
	}
	if len(resolved) != 1 || resolved[0] != "/item" {
		t.Fatalf("NOTATION resolver calls = %q, want [/item]", resolved)
	}

	same, err := p.Validate(notation, "item", resolve, NeedCanonical|NeedIdentity, nil)
	if err != nil {
		t.Fatalf("same unqualified NOTATION = %v", err)
	}
	other, err := p.Validate(notation, "other", resolve, NeedCanonical|NeedIdentity, nil)
	if err != nil {
		t.Fatalf("different declared unqualified NOTATION = %v", err)
	}
	if !item.Equal(same) || item.IdentityKey() != same.IdentityKey() {
		t.Fatalf("same NOTATION values differ: %q vs %q", item.IdentityKey(), same.IdentityKey())
	}
	if item.Equal(other) || item.IdentityKey() == other.IdentityKey() {
		t.Fatalf("different NOTATION values share identity: %q", item.IdentityKey())
	}

	undeclared := Resolver{Notation: func(string, string) bool { return false }}
	if _, err := p.Validate(notation, "item", undeclared, 0, nil); err == nil {
		t.Fatal("undeclared unqualified NOTATION was accepted")
	}
	if _, err := p.Validate(notation, "p:item", resolve, 0, nil); err == nil {
		t.Fatal("prefixed NOTATION was accepted without a QName resolver")
	}
}

func TestIncrementalBuilderValidatesCompletedTypesBeforeSeal(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	base, err := b.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	derived, err := b.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err = b.Complete(derived, TypeSpec{
		Variety: Atomic, Primitive: PrimitiveDecimal,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: base, ListItem: NoType,
	}); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Complete before base = %v, want ErrMetadata", err)
	}
	if err = b.Complete(base, TypeSpec{
		Variety: Atomic, Primitive: PrimitiveDecimal,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Validate(base, "2", Resolver{}, NeedCanonical, nil); err != nil {
		t.Fatalf("Validate completed base = %v", err)
	}
	if err = b.Complete(derived, TypeSpec{
		Variety: Atomic, Primitive: PrimitiveDecimal,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: base, ListItem: NoType,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Validate(derived, "2", Resolver{}, 0, nil); err != nil {
		t.Fatalf("Validate completed derived = %v", err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Validate(derived, "2", Resolver{}, 0, nil); err != nil {
		t.Fatalf("Validate sealed derived = %v", err)
	}
}

func TestUnionPreservesEarlierUnsupportedMemberError(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	entity, ok := BuiltinTypeID("ENTITY")
	if !ok {
		t.Fatal("missing ENTITY builtin")
	}
	id, err := b.Add(TypeSpec{
		Variety: Union, Union: []TypeID{entity, BuiltinType(PrimitiveDecimal)},
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
	if _, err := p.Validate(id, "name", Resolver{}, 0, nil); !IsUnsupported(err) {
		t.Fatalf("union error = %v, want unsupported member error", err)
	}
}
