package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func Benchmark_StepModel_DFA_Exact(b *testing.B) {
	fx := buildModelFixture(b)
	fx.schema.Models.DFA = make([]runtime.DFAModel, 2)
	fx.schema.Models.DFA[1] = runtime.DFAModel{
		Start: 0,
		States: []runtime.DFAState{
			{Accept: false, TransOff: 0, TransLen: 1},
			{Accept: true},
		},
		Transitions: []runtime.DFATransition{
			{Sym: fx.symA, Next: 1, Elem: fx.elemA},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelDFA, ID: 1}
	sess := NewSession(fx.schema)
	state, err := sess.InitModelState(ref)
	if err != nil {
		b.Fatalf("InitModelState: %v", err)
	}
	nsBytes := []byte("urn:test")

	b.ReportAllocs()
	for b.Loop() {
		state.DFA = 0
		match, err := sess.StepModel(ref, &state, fx.symA, fx.ns, nsBytes)
		if err != nil {
			b.Fatalf("StepModel: %v", err)
		}
		if match.Kind != MatchElem || match.Elem != fx.elemA {
			b.Fatalf("match = %+v, want elem %d", match, fx.elemA)
		}
	}
}

func Benchmark_StepModel_NFA_Exact(b *testing.B) {
	fx := buildModelFixture(b)
	fx.schema.Models.NFA = make([]runtime.NFAModel, 2)
	fx.schema.Models.NFA[1] = buildNFASequence(fx.symA, fx.symB, fx.elemA, fx.elemB)
	ref := runtime.ModelRef{Kind: runtime.ModelNFA, ID: 1}
	sess := NewSession(fx.schema)
	state, err := sess.InitModelState(ref)
	if err != nil {
		b.Fatalf("InitModelState: %v", err)
	}
	nsBytes := []byte("urn:test")

	b.ReportAllocs()
	for b.Loop() {
		bitsetZero(state.NFA)
		match, err := sess.StepModel(ref, &state, fx.symA, fx.ns, nsBytes)
		if err != nil {
			b.Fatalf("StepModel: %v", err)
		}
		if match.Kind != MatchElem || match.Elem != fx.elemA {
			b.Fatalf("match = %+v, want elem %d", match, fx.elemA)
		}
	}
}

func Benchmark_StepModel_All_Exact(b *testing.B) {
	fx := buildModelFixture(b)
	fx.schema.Models.All = make([]runtime.AllModel, 2)
	fx.schema.Models.All[1] = runtime.AllModel{
		Members: []runtime.AllMember{
			{Elem: fx.elemA},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelAll, ID: 1}
	sess := NewSession(fx.schema)
	state, err := sess.InitModelState(ref)
	if err != nil {
		b.Fatalf("InitModelState: %v", err)
	}
	nsBytes := []byte("urn:test")

	b.ReportAllocs()
	for b.Loop() {
		state.All[0] = 0
		state.AllCount = 0
		match, err := sess.StepModel(ref, &state, fx.symA, fx.ns, nsBytes)
		if err != nil {
			b.Fatalf("StepModel: %v", err)
		}
		if match.Kind != MatchElem || match.Elem != fx.elemA {
			b.Fatalf("match = %+v, want elem %d", match, fx.elemA)
		}
	}
}
