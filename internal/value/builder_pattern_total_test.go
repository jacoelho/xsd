package value_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

// These are guards: Validate and Complete were already charged. The
// compile-time proof that no uncharged path exists is the absence of any
// Builder method that evaluates without the aggregate account.

const patternTotalUnit = 5_005 // 1,000 ASCII runes against [a-z]+

func patternItemType(t *testing.T, builder *value.Builder) value.TypeID {
	t.Helper()
	pattern, err := xsdregex.Compile("[a-z]+")
	if err != nil {
		t.Fatal(err)
	}
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.Complete(id, value.TypeSpec{
		Variety: value.Atomic, Primitive: value.PrimitiveString,
		Whitespace: value.WhitespacePreserve, WhitespacePresent: true,
		Base: value.BuiltinType(value.PrimitiveString), ListItem: value.NoType,
		Facets: value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}, Present: value.FacetPattern},
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestBuilderValidateChargesAggregatePatternAccount(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: patternTotalUnit})
	id := patternItemType(t, builder)
	input := strings.Repeat("a", 1_000)
	if _, err := builder.Validate(id, input, value.Resolver{}, 0, nil); err != nil {
		t.Fatalf("first Validate() error = %v", err)
	}
	_, err := builder.Validate(id, input, value.Resolver{}, 0, nil)
	if !errors.Is(err, value.ErrPatternTotalLimit) || errors.Is(err, value.ErrPatternLimit) {
		t.Fatalf("second Validate() error = %v, want ErrPatternTotalLimit and not ErrPatternLimit", err)
	}
}

func TestBuilderCompleteChargesAggregatePatternAccount(t *testing.T) {
	t.Parallel()

	complete := func(total uint64) error {
		builder := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: total})
		item := patternItemType(t, builder)
		list, err := builder.Reserve()
		if err != nil {
			t.Fatal(err)
		}
		if listErr := builder.Complete(list, value.TypeSpec{
			Variety: value.List, Primitive: value.PrimitiveString,
			Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
			Base: value.NoType, ListItem: item,
		}); listErr != nil {
			t.Fatal(listErr)
		}
		derived, err := builder.Reserve()
		if err != nil {
			t.Fatal(err)
		}
		return builder.Complete(derived, value.TypeSpec{
			Variety: value.List, Primitive: value.PrimitiveString,
			Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
			Base: list, ListItem: item,
			Facets: value.FacetSpec{
				Enumeration: []value.LiteralSpec{{Lexical: strings.Repeat("a", 1_000), Type: list}},
				Present:     value.FacetEnumeration,
			},
		})
	}
	// The item type's own Complete does no pattern work, so the measured
	// single-evaluation cost of the literal is the unit.
	if err := complete(patternTotalUnit); err != nil {
		t.Fatalf("Complete() at measured cost: %v", err)
	}
	if err := complete(patternTotalUnit - 1); !errors.Is(err, value.ErrPatternTotalLimit) {
		t.Fatalf("Complete() at cost-1 error = %v, want ErrPatternTotalLimit", err)
	}
}
