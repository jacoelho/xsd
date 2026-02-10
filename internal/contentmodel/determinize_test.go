package contentmodel

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestDeterminizeSequence(t *testing.T) {
	a := elem("a", 1, 1)
	b := elem("b", 1, 1)
	group := sequence(a, b)

	glu, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}

	matchers := []runtime.PosMatcher{
		{Kind: runtime.PosExact, Sym: 1, Elem: 10},
		{Kind: runtime.PosExact, Sym: 2, Elem: 20},
	}

	compiled, err := Compile(glu, matchers, Limits{MaxDFAStates: 16})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if compiled.Kind != runtime.ModelDFA {
		t.Fatalf("model kind = %v, want DFA", compiled.Kind)
	}

	dfa := compiled.DFA
	if len(dfa.States) != 3 {
		t.Fatalf("states = %d, want 3", len(dfa.States))
	}
	if dfa.States[0].Accept || dfa.States[1].Accept || !dfa.States[2].Accept {
		t.Fatalf("unexpected accept flags: %+v", []bool{dfa.States[0].Accept, dfa.States[1].Accept, dfa.States[2].Accept})
	}

	trans0 := dfa.Transitions[dfa.States[0].TransOff : dfa.States[0].TransOff+dfa.States[0].TransLen]
	if len(trans0) != 1 {
		t.Fatalf("state0 transitions = %d, want 1", len(trans0))
	}
	if trans0[0].Sym != 1 || trans0[0].Next != 1 || trans0[0].Elem != 10 {
		t.Fatalf("state0 transition = %+v", trans0[0])
	}

	trans1 := dfa.Transitions[dfa.States[1].TransOff : dfa.States[1].TransOff+dfa.States[1].TransLen]
	if len(trans1) != 1 {
		t.Fatalf("state1 transitions = %d, want 1", len(trans1))
	}
	if trans1[0].Sym != 2 || trans1[0].Next != 2 || trans1[0].Elem != 20 {
		t.Fatalf("state1 transition = %+v", trans1[0])
	}

	trans2 := dfa.Transitions[dfa.States[2].TransOff : dfa.States[2].TransOff+dfa.States[2].TransLen]
	if len(trans2) != 0 {
		t.Fatalf("state2 transitions = %d, want 0", len(trans2))
	}

	if dfa.States[0].WildLen != 0 || dfa.States[1].WildLen != 0 || dfa.States[2].WildLen != 0 {
		t.Fatalf("unexpected wildcard edges")
	}
}

func TestDeterminizeFallbackToNFA(t *testing.T) {
	a := elem("a", 1, 1)
	b := elem("b", 1, 1)
	group := sequence(a, b)

	glu, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}

	matchers := []runtime.PosMatcher{
		{Kind: runtime.PosExact, Sym: 1, Elem: 10},
		{Kind: runtime.PosExact, Sym: 2, Elem: 20},
	}

	compiled, err := Compile(glu, matchers, Limits{MaxDFAStates: 1})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if compiled.Kind != runtime.ModelNFA {
		t.Fatalf("model kind = %v, want NFA", compiled.Kind)
	}
	if len(compiled.NFA.Matchers) != len(matchers) {
		t.Fatalf("matchers = %d, want %d", len(compiled.NFA.Matchers), len(matchers))
	}
	if compiled.NFA.Start.Len == 0 {
		t.Fatalf("expected non-empty start set")
	}
}

func TestDeterminizeWildcardEdges(t *testing.T) {
	anyElem := &model.AnyElement{
		MinOccurs: model.OccursFromInt(1),
		MaxOccurs: model.OccursFromInt(1),
	}

	glu, err := BuildGlushkov(anyElem)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}

	matchers := []runtime.PosMatcher{
		{Kind: runtime.PosWildcard, Rule: 3},
	}

	compiled, err := Compile(glu, matchers, Limits{MaxDFAStates: 8})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if compiled.Kind != runtime.ModelDFA {
		t.Fatalf("model kind = %v, want DFA", compiled.Kind)
	}
	if len(compiled.DFA.Wildcards) != 1 {
		t.Fatalf("wildcard edges = %d, want 1", len(compiled.DFA.Wildcards))
	}
	state0 := compiled.DFA.States[0]
	if state0.WildLen != 1 {
		t.Fatalf("state0 wildcard len = %d, want 1", state0.WildLen)
	}
	if compiled.DFA.Wildcards[state0.WildOff].Rule != 3 {
		t.Fatalf("wildcard rule = %d, want 3", compiled.DFA.Wildcards[state0.WildOff].Rule)
	}
}
