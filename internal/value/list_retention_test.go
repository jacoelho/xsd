package value

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

func TestListEnumerationRetainsCompleteValueOrNoItems(t *testing.T) {
	builder := NewBuilder(BuilderOptions{})
	listID, err := builder.Add(TypeSpec{
		Variety:           List,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          builtinString,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{
			Lexical: "a",
			Type:    NoType,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	typeDef, ok := program.typeDef(listID)
	if !ok {
		t.Fatal("list type metadata unavailable")
	}
	if typeDef.facets.enumItemLimit != 1 {
		t.Fatalf("enum item limit = %d, want 1", typeDef.facets.enumItemLimit)
	}

	for _, test := range []struct {
		name       string
		lexical    string
		wantCount  uint32
		wantLength int
		wantCap    int
	}{
		{name: "within demand", lexical: "a", wantCount: 1, wantLength: 1, wantCap: 1},
		{name: "above demand", lexical: "a a", wantCount: 2, wantLength: 0, wantCap: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			budget, err := newEvaluationBudget(1 << 20)
			if err != nil {
				t.Fatal(err)
			}
			var got parsedValue
			if _, err := program.eval(listID, test.lexical, evalOptions{work: &budget}, &got); err != nil {
				t.Fatalf("eval(%q): %v", test.lexical, err)
			}
			if got.count != test.wantCount || len(got.items) != test.wantLength || cap(got.items) != test.wantCap {
				t.Fatalf("list state = count %d, len %d, cap %d; want count %d, len %d, cap %d", got.count, len(got.items), cap(got.items), test.wantCount, test.wantLength, test.wantCap)
			}
		})
	}
}

func TestListEnumerationStillValidatesItemsPastRetentionBound(t *testing.T) {
	pattern, err := xsdregex.Compile("a", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	builder := NewBuilder(BuilderOptions{})
	itemID, err := builder.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets:            FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	listID, err := builder.Add(TypeSpec{
		Variety:           List,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          itemID,
		Facets:            FacetSpec{Enumeration: []LiteralSpec{{Lexical: "a", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	_, err = program.Validate(listID, "a b", Resolver{}, 0, 16<<20, nil)
	if err == nil || !strings.Contains(err.Error(), "pattern facet failed") {
		t.Fatalf("Validate oversized list error = %v, want item pattern failure", err)
	}
}

func TestUnionListMemberFirstSuccessCannotFallThroughAfterOuterEnumeration(t *testing.T) {
	builder, listID := listWithPatternAndEnumeration(t, "0[0-9]", "01")
	unionID, err := builder.Add(TypeSpec{
		Variety:           Union,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Union:             []TypeID{listID, builtinInt},
		Facets:            FacetSpec{Enumeration: []LiteralSpec{{Lexical: "1", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(unionID, "01", Resolver{}, 0, 16<<20, nil); err == nil {
		t.Fatal("Validate(\"01\") succeeded; list selection must fail the outer enumeration")
	}
}

func TestUnionOuterScalarEnumerationDoesNotFallThroughListFirstSuccess(t *testing.T) {
	builder, listID := listWithPattern(t, "0[0-9]")
	unionID, err := builder.Add(TypeSpec{
		Variety:           Union,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Union:             []TypeID{listID, builtinInt},
		Facets:            FacetSpec{Enumeration: []LiteralSpec{{Lexical: "1", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(unionID, "01", Resolver{}, 0, 16<<20, nil); err == nil {
		t.Fatal("Validate(\"01\") succeeded; outer enumeration must see the selected list value")
	}
}

func TestUnionFallsBackAfterListEnumerationFailure(t *testing.T) {
	builder, listID := listWithPatternAndEnumeration(t, "0[0-9]", "01")
	unionID, err := builder.Add(TypeSpec{
		Variety:           Union,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Union:             []TypeID{listID, builtinInt},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	value, err := program.Validate(unionID, "02", Resolver{}, 0, 16<<20, nil)
	if err != nil {
		t.Fatalf("Validate(\"02\"): %v", err)
	}
	if value.SelectedType() != builtinInt {
		t.Fatalf("SelectedType() = %d, want builtin int %d", value.SelectedType(), builtinInt)
	}
}

func TestUnionScalarEnumerationAcceptsFallbackAfterListEnumerationFailure(t *testing.T) {
	builder, listID := listWithPatternAndEnumeration(t, "0[0-9]", "01")
	unionID, err := builder.Add(TypeSpec{
		Variety:           Union,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Union:             []TypeID{listID, builtinInt},
		Facets:            FacetSpec{Enumeration: []LiteralSpec{{Lexical: "2", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	value, err := program.Validate(unionID, "02", Resolver{}, 0, 16<<20, nil)
	if err != nil {
		t.Fatalf("Validate(\"02\"): %v", err)
	}
	if value.SelectedType() != builtinInt {
		t.Fatalf("SelectedType() = %d, want builtin int %d", value.SelectedType(), builtinInt)
	}
}

func listWithPatternAndEnumeration(t *testing.T, patternSource, enumeration string) (*Builder, TypeID) {
	t.Helper()
	literal := &LiteralSpec{Lexical: enumeration, Type: NoType}
	return listWithPatternSpec(t, patternSource, literal)
}

func listWithPattern(t *testing.T, patternSource string) (*Builder, TypeID) {
	t.Helper()
	return listWithPatternSpec(t, patternSource, nil)
}

func listWithPatternSpec(t *testing.T, patternSource string, literal *LiteralSpec) (*Builder, TypeID) {
	t.Helper()
	pattern, err := xsdregex.Compile(patternSource, xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	builder := NewBuilder(BuilderOptions{})
	itemID, err := builder.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets:            FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var facets FacetSpec
	if literal != nil {
		facets.Enumeration = []LiteralSpec{*literal}
	}
	listID, err := builder.Add(TypeSpec{
		Variety:           List,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          itemID,
		Facets:            facets,
	})
	if err != nil {
		t.Fatal(err)
	}
	return builder, listID
}
