package validator

import (
	"strings"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

type modelFixture struct {
	schema *runtime.Schema
	ns     runtime.NamespaceID
	symA   runtime.SymbolID
	symB   runtime.SymbolID
	symC   runtime.SymbolID
	elemA  runtime.ElemID
	elemB  runtime.ElemID
	elemC  runtime.ElemID
}

func buildModelFixture(tb testing.TB) modelFixture {
	tb.Helper()
	builder := runtime.NewBuilder()
	ns := mustInternNamespace(tb, builder, []byte("urn:test"))
	symA := mustInternSymbol(tb, builder, ns, []byte("a"))
	symB := mustInternSymbol(tb, builder, ns, []byte("b"))
	symC := mustInternSymbol(tb, builder, ns, []byte("c"))
	schema, err := builder.Build()
	if err != nil {
		tb.Fatalf("Build() error = %v", err)
	}

	schema.Elements = make([]runtime.Element, 4)
	schema.Elements[1] = runtime.Element{Name: symA}
	schema.Elements[2] = runtime.Element{Name: symB}
	schema.Elements[3] = runtime.Element{Name: symC, SubstHead: 1}
	schema.GlobalElements = make([]runtime.ElemID, schema.Symbols.Count()+1)
	schema.GlobalElements[symA] = 1
	schema.GlobalElements[symB] = 2
	schema.GlobalElements[symC] = 3

	return modelFixture{
		schema: schema,
		ns:     ns,
		symA:   symA,
		symB:   symB,
		symC:   symC,
		elemA:  1,
		elemB:  2,
		elemC:  3,
	}
}

func TestModelStateDFASequence(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.DFA = make([]runtime.DFAModel, 2)
	fx.schema.Models.DFA[1] = runtime.DFAModel{
		Start: 0,
		States: []runtime.DFAState{
			{Accept: false, TransOff: 0, TransLen: 1},
			{Accept: false, TransOff: 1, TransLen: 1},
			{Accept: true, TransOff: 2, TransLen: 0},
		},
		Transitions: []runtime.DFATransition{
			{Sym: fx.symA, Next: 1, Elem: fx.elemA},
			{Sym: fx.symB, Next: 2, Elem: fx.elemB},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelDFA, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	match, err := sess.StepModel(ref, &state, fx.symA, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel a: %v", err)
	}
	if match.Kind != MatchElem || match.Elem != fx.elemA {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemA)
	}
	if err := sess.AcceptModel(ref, &state); err == nil {
		t.Fatalf("expected accept failure after single step")
	}

	match, err = sess.StepModel(ref, &state, fx.symB, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel b: %v", err)
	}
	if match.Kind != MatchElem || match.Elem != fx.elemB {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemB)
	}
	if err := sess.AcceptModel(ref, &state); err != nil {
		t.Fatalf("AcceptModel: %v", err)
	}
}

func TestModelStateDFAWildcardMatch(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Wildcards = []runtime.WildcardRule{
		{},
		{NS: runtime.NSConstraint{Kind: runtime.NSAny}, PC: runtime.PCSkip},
	}
	fx.schema.Models.DFA = make([]runtime.DFAModel, 2)
	fx.schema.Models.DFA[1] = runtime.DFAModel{
		Start: 0,
		States: []runtime.DFAState{
			{Accept: false, WildOff: 0, WildLen: 1},
			{Accept: true},
		},
		Wildcards: []runtime.DFAWildcardEdge{
			{Rule: 1, Next: 1},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelDFA, ID: 1}
	sess := NewSession(fx.schema)
	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	match, err := sess.StepModel(ref, &state, 0, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel wildcard: %v", err)
	}
	if match.Kind != MatchWildcard || match.Wildcard != 1 {
		t.Fatalf("match = %+v, want wildcard 1", match)
	}
}

func TestModelStateDFAWildcardAmbiguous(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Wildcards = []runtime.WildcardRule{
		{},
		{NS: runtime.NSConstraint{Kind: runtime.NSAny}, PC: runtime.PCSkip},
		{NS: runtime.NSConstraint{Kind: runtime.NSAny}, PC: runtime.PCSkip},
	}
	fx.schema.Models.DFA = make([]runtime.DFAModel, 2)
	fx.schema.Models.DFA[1] = runtime.DFAModel{
		Start: 0,
		States: []runtime.DFAState{
			{Accept: false, WildOff: 0, WildLen: 2},
			{Accept: true},
		},
		Wildcards: []runtime.DFAWildcardEdge{
			{Rule: 1, Next: 1},
			{Rule: 2, Next: 1},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelDFA, ID: 1}
	sess := NewSession(fx.schema)
	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	if _, err := sess.StepModel(ref, &state, 0, fx.ns, []byte("urn:test")); err == nil {
		t.Fatalf("expected ambiguous wildcard error")
	}
}

func TestModelStateDFANoMatchErrorCode(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.DFA = make([]runtime.DFAModel, 2)
	fx.schema.Models.DFA[1] = runtime.DFAModel{
		Start: 0,
		States: []runtime.DFAState{
			{Accept: true},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelDFA, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	_, err = sess.StepModel(ref, &state, fx.symA, fx.ns, []byte("urn:test"))
	if err == nil {
		t.Fatalf("expected no-match error")
	}
	code, ok := validationErrorInfo(err)
	if !ok || code != xsderrors.ErrUnexpectedElement {
		t.Fatalf("error code = %v, want %v", code, xsderrors.ErrUnexpectedElement)
	}
}

func TestModelStateDFANoMatchIncludesExpectedAndActual(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.DFA = make([]runtime.DFAModel, 2)
	fx.schema.Models.DFA[1] = runtime.DFAModel{
		Start: 0,
		States: []runtime.DFAState{
			{Accept: false, TransOff: 0, TransLen: 1},
		},
		Transitions: []runtime.DFATransition{
			{Sym: fx.symA, Next: 0, Elem: fx.elemA},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelDFA, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	_, err = sess.StepModel(ref, &state, fx.symB, fx.ns, []byte("urn:test"))
	if err == nil {
		t.Fatalf("expected no-match error")
	}
	details := validationErrorDetails(err)
	if !details.ok {
		t.Fatalf("validation details not found: %v", err)
	}
	if details.code != xsderrors.ErrUnexpectedElement {
		t.Fatalf("error code = %v, want %v", details.code, xsderrors.ErrUnexpectedElement)
	}
	if !containsExpectedLocal(details.expected, "a") {
		t.Fatalf("expected names = %v, want local name a", details.expected)
	}
	if !containsExpectedLocal([]string{details.actual}, "b") {
		t.Fatalf("actual = %q, want local name b", details.actual)
	}
}

func TestModelStateDFAAcceptIncludesExpected(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.DFA = make([]runtime.DFAModel, 2)
	fx.schema.Models.DFA[1] = runtime.DFAModel{
		Start: 0,
		States: []runtime.DFAState{
			{Accept: false, TransOff: 0, TransLen: 1},
			{Accept: true},
		},
		Transitions: []runtime.DFATransition{
			{Sym: fx.symB, Next: 1, Elem: fx.elemB},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelDFA, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	err = sess.AcceptModel(ref, &state)
	if err == nil {
		t.Fatalf("expected accept failure")
	}
	details := validationErrorDetails(err)
	if !details.ok {
		t.Fatalf("validation details not found: %v", err)
	}
	if details.code != xsderrors.ErrContentModelInvalid {
		t.Fatalf("error code = %v, want %v", details.code, xsderrors.ErrContentModelInvalid)
	}
	if !containsExpectedLocal(details.expected, "b") {
		t.Fatalf("expected names = %v, want local name b", details.expected)
	}
}

func TestModelStateNFASequence(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.NFA = make([]runtime.NFAModel, 2)
	fx.schema.Models.NFA[1] = buildNFASequence(fx.symA, fx.symB, fx.elemA, fx.elemB)
	ref := runtime.ModelRef{Kind: runtime.ModelNFA, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	match, err := sess.StepModel(ref, &state, fx.symA, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel a: %v", err)
	}
	if match.Kind != MatchElem || match.Elem != fx.elemA {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemA)
	}
	if err := sess.AcceptModel(ref, &state); err == nil {
		t.Fatalf("expected accept failure after single step")
	}

	match, err = sess.StepModel(ref, &state, fx.symB, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel b: %v", err)
	}
	if match.Kind != MatchElem || match.Elem != fx.elemB {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemB)
	}
	if err := sess.AcceptModel(ref, &state); err != nil {
		t.Fatalf("AcceptModel: %v", err)
	}
}

func TestModelStateNFANullableAcceptsEmpty(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.NFA = make([]runtime.NFAModel, 2)
	fx.schema.Models.NFA[1] = buildNFANullable(fx.symA, fx.elemA)
	ref := runtime.ModelRef{Kind: runtime.ModelNFA, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	if err := sess.AcceptModel(ref, &state); err != nil {
		t.Fatalf("AcceptModel: %v", err)
	}
}

func TestModelStateNFAAmbiguousErrorCode(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.NFA = make([]runtime.NFAModel, 2)
	fx.schema.Models.NFA[1] = runtime.NFAModel{
		Bitsets: runtime.BitsetBlob{
			Words: []uint64{3},
		},
		Start: runtime.BitsetRef{Off: 0, Len: 1},
		Matchers: []runtime.PosMatcher{
			{Kind: runtime.PosExact, Sym: fx.symA, Elem: fx.elemA},
			{Kind: runtime.PosExact, Sym: fx.symA, Elem: fx.elemB},
		},
		FollowLen: 2,
		Follow: []runtime.BitsetRef{
			{},
			{},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelNFA, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	_, err = sess.StepModel(ref, &state, fx.symA, fx.ns, []byte("urn:test"))
	if err == nil {
		t.Fatalf("expected ambiguous-match error")
	}
	code, ok := validationErrorInfo(err)
	if !ok || code != xsderrors.ErrContentModelInvalid {
		t.Fatalf("error code = %v, want %v", code, xsderrors.ErrContentModelInvalid)
	}
}

func TestModelStateAllGroup(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.All = make([]runtime.AllModel, 2)
	fx.schema.Models.AllSubst = []runtime.ElemID{fx.elemA, fx.elemC}
	fx.schema.Models.All[1] = runtime.AllModel{
		MinOccurs: 1,
		Mixed:     false,
		Members: []runtime.AllMember{
			{Elem: fx.elemA, Optional: false, AllowsSubst: true, SubstOff: 0, SubstLen: 2},
			{Elem: fx.elemB, Optional: true},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelAll, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	match, err := sess.StepModel(ref, &state, fx.symB, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel b: %v", err)
	}
	if match.Kind != MatchElem || match.Elem != fx.elemB {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemB)
	}
	match, err = sess.StepModel(ref, &state, fx.symA, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel a: %v", err)
	}
	if match.Kind != MatchElem || match.Elem != fx.elemA {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemA)
	}
	if err := sess.AcceptModel(ref, &state); err != nil {
		t.Fatalf("AcceptModel: %v", err)
	}

	state, _ = sess.InitModelState(ref)
	match, err = sess.StepModel(ref, &state, fx.symC, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel subst: %v", err)
	}
	if match.Kind != MatchElem || match.Elem != fx.elemC {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemC)
	}
	if err := sess.AcceptModel(ref, &state); err != nil {
		t.Fatalf("AcceptModel: %v", err)
	}

	state, _ = sess.InitModelState(ref)
	if _, err := sess.StepModel(ref, &state, fx.symA, fx.ns, []byte("urn:test")); err != nil {
		t.Fatalf("StepModel a: %v", err)
	}
	if _, err := sess.StepModel(ref, &state, fx.symA, fx.ns, []byte("urn:test")); err == nil {
		t.Fatalf("expected duplicate element error")
	}

	state, _ = sess.InitModelState(ref)
	if _, err := sess.StepModel(ref, &state, fx.symB, fx.ns, []byte("urn:test")); err != nil {
		t.Fatalf("StepModel b: %v", err)
	}
	if err := sess.AcceptModel(ref, &state); err == nil {
		t.Fatalf("expected missing required error")
	}

	fx.schema.Models.All[1] = runtime.AllModel{
		MinOccurs: 0,
		Mixed:     false,
		Members: []runtime.AllMember{
			{Elem: fx.elemA, Optional: false},
		},
	}
	state, _ = sess.InitModelState(ref)
	if err := sess.AcceptModel(ref, &state); err != nil {
		t.Fatalf("AcceptModel empty: %v", err)
	}
}

func TestModelStateAllMissingRequiredIncludesExpectedMembers(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.All = make([]runtime.AllModel, 2)
	fx.schema.Models.AllSubst = []runtime.ElemID{fx.elemA, fx.elemC}
	fx.schema.Models.All[1] = runtime.AllModel{
		MinOccurs: 1,
		Members: []runtime.AllMember{
			{Elem: fx.elemA, Optional: false, AllowsSubst: true, SubstOff: 0, SubstLen: 2},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelAll, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	err = sess.AcceptModel(ref, &state)
	if err == nil {
		t.Fatalf("expected required-element error")
	}
	details := validationErrorDetails(err)
	if !details.ok {
		t.Fatalf("validation details not found: %v", err)
	}
	if details.code != xsderrors.ErrRequiredElementMissing {
		t.Fatalf("error code = %v, want %v", details.code, xsderrors.ErrRequiredElementMissing)
	}
	if !containsExpectedLocal(details.expected, "a") {
		t.Fatalf("expected names = %v, want local name a", details.expected)
	}
	if !containsExpectedLocal(details.expected, "c") {
		t.Fatalf("expected names = %v, want substitution member c", details.expected)
	}
}

func TestModelStateAllAmbiguousErrorCode(t *testing.T) {
	fx := buildModelFixture(t)
	fx.schema.Models.All = make([]runtime.AllModel, 2)
	fx.schema.Models.AllSubst = []runtime.ElemID{
		fx.elemA, fx.elemC,
		fx.elemB, fx.elemC,
	}
	fx.schema.Models.All[1] = runtime.AllModel{
		Members: []runtime.AllMember{
			{Elem: fx.elemA, AllowsSubst: true, SubstOff: 0, SubstLen: 2},
			{Elem: fx.elemB, AllowsSubst: true, SubstOff: 2, SubstLen: 2},
		},
	}
	ref := runtime.ModelRef{Kind: runtime.ModelAll, ID: 1}
	sess := NewSession(fx.schema)

	state, err := sess.InitModelState(ref)
	if err != nil {
		t.Fatalf("InitModelState: %v", err)
	}
	_, err = sess.StepModel(ref, &state, fx.symC, fx.ns, []byte("urn:test"))
	if err == nil {
		t.Fatalf("expected ambiguous-match error")
	}
	code, ok := validationErrorInfo(err)
	if !ok || code != xsderrors.ErrContentModelInvalid {
		t.Fatalf("error code = %v, want %v", code, xsderrors.ErrContentModelInvalid)
	}
}

func containsExpectedLocal(values []string, local string) bool {
	for _, value := range values {
		if value == local {
			return true
		}
		if strings.HasSuffix(value, "}"+local) {
			return true
		}
	}
	return false
}

func buildNFASequence(symA, symB runtime.SymbolID, elemA, elemB runtime.ElemID) runtime.NFAModel {
	blob := runtime.BitsetBlob{Words: []uint64{1, 2, 2}}
	return runtime.NFAModel{
		Bitsets:   blob,
		Start:     runtime.BitsetRef{Off: 0, Len: 1},
		Accept:    runtime.BitsetRef{Off: 1, Len: 1},
		Nullable:  false,
		FollowOff: 0,
		FollowLen: 2,
		Matchers: []runtime.PosMatcher{
			{Kind: runtime.PosExact, Sym: symA, Elem: elemA},
			{Kind: runtime.PosExact, Sym: symB, Elem: elemB},
		},
		Follow: []runtime.BitsetRef{
			{Off: 2, Len: 1},
			{},
		},
	}
}

func buildNFANullable(sym runtime.SymbolID, elem runtime.ElemID) runtime.NFAModel {
	blob := runtime.BitsetBlob{Words: []uint64{1}}
	return runtime.NFAModel{
		Bitsets:   blob,
		Start:     runtime.BitsetRef{Off: 0, Len: 1},
		Accept:    runtime.BitsetRef{Off: 0, Len: 1},
		Nullable:  true,
		FollowOff: 0,
		FollowLen: 1,
		Matchers: []runtime.PosMatcher{
			{Kind: runtime.PosExact, Sym: sym, Elem: elem},
		},
		Follow: []runtime.BitsetRef{
			{},
		},
	}
}
