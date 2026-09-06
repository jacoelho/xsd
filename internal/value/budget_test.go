package value

import (
	"errors"
	"math"
	"testing"
)

func TestProgramValueWorkLimitIsRequiredBeforeFastPaths(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Validate(builtinString, "", Resolver{}, 0, 0, nil); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Validate zero work limit = %v, want ErrMetadata", err)
	}
	if _, err := p.ValidateBytes(builtinString, nil, Resolver{}, 0, 0, nil); !errors.Is(err, ErrMetadata) {
		t.Fatalf("ValidateBytes zero work limit = %v, want ErrMetadata", err)
	}
}

func TestProgramValueWorkLimitBoundsPlainStringAndResetsPerCall(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Validate(builtinString, "x", Resolver{}, 0, 1, nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("plain string below scalar charge = %v, want ErrLimit", err)
	}
	if _, err := p.Validate(builtinString, "x", Resolver{}, 0, 2, nil); err != nil {
		t.Fatalf("plain string at scalar charge = %v", err)
	}
	if _, err := p.Validate(builtinString, "x", Resolver{}, 0, 2, nil); err != nil {
		t.Fatalf("subsequent value reused work usage = %v", err)
	}
}

func TestBuilderValueValidationUsesFreshConstructionBudget(t *testing.T) {
	builder := NewBuilder(BuilderOptions{MaxConstructionWork: 2})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.Complete(id, TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	}); err != nil {
		t.Fatal(err)
	}
	for call := range 2 {
		if _, err := builder.Validate(id, "x", Resolver{}, 0, nil); err != nil {
			t.Fatalf("Validate() call %d = %v", call+1, err)
		}
	}
	if _, err := builder.Validate(id, "xx", Resolver{}, 0, nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("Validate() above construction work limit = %v, want ErrLimit", err)
	}
}

func TestBuilderFacetBatchSharesConstructionBudget(t *testing.T) {
	makeSpec := func() TypeSpec {
		return TypeSpec{
			Variety: Atomic, Primitive: PrimitiveDecimal,
			Whitespace: WhitespaceCollapse, WhitespacePresent: true,
			Base: NoType, ListItem: NoType,
			Facets: FacetSpec{
				MinInclusive: BoundFacet{
					LiteralSpec: LiteralSpec{Lexical: "1", Type: NoType},
					Present:     true,
				},
				Enumeration: []LiteralSpec{{Lexical: "1", Type: NoType}},
			},
		}
	}

	for _, test := range []struct {
		name  string
		limit uint64
		want  error
	}{
		{name: "below", limit: 3, want: ErrLimit},
		{name: "at", limit: 4},
		{name: "above", limit: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			builder := NewBuilder(BuilderOptions{MaxConstructionWork: test.limit})
			if _, err := builder.Add(makeSpec()); err != nil {
				t.Fatal(err)
			}
			_, err := builder.Seal()
			if test.want != nil {
				if !errors.Is(err, test.want) {
					t.Fatalf("Seal() error = %v, want %v", err, test.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("Seal() error = %v", err)
			}
		})
	}
}

func TestBuilderCompleteGetsFreshFacetBudgetAfterFailure(t *testing.T) {
	builder := NewBuilder(BuilderOptions{MaxConstructionWork: 4})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	first := TypeSpec{
		Variety: Atomic, Primitive: PrimitiveDecimal,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{
			MinInclusive: BoundFacet{
				LiteralSpec: LiteralSpec{Lexical: "1", Type: NoType},
				Present:     true,
			},
			Enumeration: []LiteralSpec{{Lexical: "1", Type: NoType}, {Lexical: "2", Type: NoType}},
		},
	}
	if err := builder.Complete(id, first); !errors.Is(err, ErrLimit) {
		t.Fatalf("first Complete() error = %v, want ErrLimit", err)
	}
	second := first
	second.Facets.Enumeration = second.Facets.Enumeration[:1]
	if err := builder.Complete(id, second); err != nil {
		t.Fatalf("retry Complete() error = %v", err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("Seal() after retry error = %v", err)
	}
}

func TestEvaluationBudgetRejectsUsageOverflow(t *testing.T) {
	budget := evaluationBudget{limit: math.MaxUint64, used: math.MaxUint64}
	if err := budget.charge(0); !errors.Is(err, ErrLimit) {
		t.Fatalf("charge at exhausted MaxUint64 budget = %v, want ErrLimit", err)
	}
	budget = evaluationBudget{limit: math.MaxUint64, used: math.MaxUint64 - 1}
	if err := budget.charge(0); err != nil {
		t.Fatalf("final unit at MaxUint64 budget = %v", err)
	}
	if err := budget.charge(0); !errors.Is(err, ErrLimit) {
		t.Fatalf("charge after the final unit = %v, want ErrLimit", err)
	}
}
