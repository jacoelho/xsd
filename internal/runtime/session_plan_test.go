package runtime

import "testing"

func TestNewSessionPlanCountsNFAStateAndScratch(t *testing.T) {
	schema := &Schema{
		models: ModelsBundle{
			NFA: make([]NFAModel, 2),
		},
	}
	schema.models.NFA[1] = NFAModel{Start: BitsetRef{Len: 2}}

	plan := NewSessionPlan(schema)
	if plan.MaxModelWords != 4 {
		t.Fatalf("MaxModelWords = %d, want 4", plan.MaxModelWords)
	}
}

func TestNewSessionPlanCountsNestedLiveModelWords(t *testing.T) {
	schema := &Schema{
		globalElements: []ElemID{0, 1},
		types: []Type{
			{},
			{Kind: TypeComplex, Complex: ComplexTypeRef{ID: 1}},
			{Kind: TypeComplex, Complex: ComplexTypeRef{ID: 2}},
		},
		complexTypes: []ComplexType{
			{},
			{Model: ModelRef{Kind: ModelNFA, ID: 1}},
			{Model: ModelRef{Kind: ModelAll, ID: 1}},
		},
		elements: []Element{
			{},
			{Type: 1},
			{Type: 2},
		},
		models: ModelsBundle{
			NFA: make([]NFAModel, 2),
			All: make([]AllModel, 2),
		},
	}
	schema.models.NFA[1] = NFAModel{
		Start: BitsetRef{Len: 1},
		Matchers: []PosMatcher{
			{Kind: PosExact, Elem: 2},
		},
	}
	schema.models.All[1] = AllModel{Members: make([]AllMember, 65)}

	plan := NewSessionPlan(schema)
	if plan.MaxModelWords != 4 {
		t.Fatalf("MaxModelWords = %d, want 4", plan.MaxModelWords)
	}
}
