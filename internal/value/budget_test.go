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
	if _, admissionErr := p.ValidateAdmitted(builtinString, "x", Resolver{}, NeedCanonical, 0, nil); !errors.Is(admissionErr, ErrMetadata) {
		t.Fatalf("ValidateAdmitted zero work limit = %v, want ErrMetadata", admissionErr)
	}
	accepted, admissionErr := p.ValidateAdmitted(builtinString, "x", Resolver{}, NeedCanonical, 2, nil)
	if admissionErr != nil || accepted.Type() != builtinString || accepted.SelectedType() != NoType || accepted.CanonicalText() != "x" {
		t.Fatalf("ValidateAdmitted after rejection = type %d, selection %d, canonical %q, error %v; want string, NoType, x, nil", accepted.Type(), accepted.SelectedType(), accepted.CanonicalText(), admissionErr)
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
			reservedID, reserveErr := builder.Reserve()
			if reserveErr != nil {
				t.Fatalf("Reserve() error = %v", reserveErr)
			}
			err := builder.Complete(reservedID, makeSpec())
			if test.want != nil {
				if !errors.Is(err, test.want) {
					t.Fatalf("Complete() error = %v, want %v", err, test.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("Complete() error = %v", err)
			}
			if _, sealErr := builder.Seal(); sealErr != nil {
				t.Fatalf("Seal() after successful completion = %v", sealErr)
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

func TestTemporalWorkLimitResetsPerCall(t *testing.T) {
	p, err := NewBuilder(BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id      TypeID
		lexical string
	}{
		{builtinGYear, "1234567890123456789012345678901234567890Z"},
		{builtinTime, "12:34:56.123456789012345678901234567890Z"},
	} {
		t.Run(tc.lexical, func(t *testing.T) {
			raw := []byte(tc.lexical)
			limit := uint64(len(tc.lexical))
			for call := range 2 {
				if _, err := p.Validate(tc.id, tc.lexical, Resolver{}, 0, limit, nil); !errors.Is(err, ErrLimit) {
					t.Fatalf("string call %d error = %v, want ErrLimit", call, err)
				}
				if _, err := p.ValidateBytes(tc.id, raw, Resolver{}, 0, limit, nil); !errors.Is(err, ErrLimit) {
					t.Fatalf("bytes call %d error = %v, want ErrLimit", call, err)
				}
				if _, err := p.Validate(tc.id, tc.lexical, Resolver{}, 0, limit+1, nil); err != nil {
					t.Fatalf("string call %d at budget = %v", call, err)
				}
				if _, err := p.ValidateBytes(tc.id, raw, Resolver{}, 0, limit+1, nil); err != nil {
					t.Fatalf("bytes call %d at budget = %v", call, err)
				}
			}
		})
	}
}
