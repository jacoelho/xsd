package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
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

	setRuntimeElements(tb, schema, make([]runtime.Element, 4))
	schema.ElementTable()[1] = runtime.Element{Name: symA}
	schema.ElementTable()[2] = runtime.Element{Name: symB}
	schema.ElementTable()[3] = runtime.Element{Name: symC, SubstHead: 1}
	setRuntimeGlobalElements(tb, schema, make([]runtime.ElemID, schema.SymbolCount()+1))
	schema.GlobalElementIDs()[symA] = 1
	schema.GlobalElementIDs()[symB] = 2
	schema.GlobalElementIDs()[symC] = 3

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
	setRuntimeDFAModels(t, fx.schema, make([]runtime.DFAModel, 2))
	fx.schema.ModelBundle().DFA[1] = runtime.DFAModel{
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
	if match.Kind != StartMatchElem || match.Elem != fx.elemA {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemA)
	}
	if err := sess.AcceptModel(ref, &state); err == nil {
		t.Fatalf("expected accept failure after single step")
	}

	match, err = sess.StepModel(ref, &state, fx.symB, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel b: %v", err)
	}
	if match.Kind != StartMatchElem || match.Elem != fx.elemB {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemB)
	}
	if err := sess.AcceptModel(ref, &state); err != nil {
		t.Fatalf("AcceptModel: %v", err)
	}
}

func TestModelStateDFAWildcardMatch(t *testing.T) {
	fx := buildModelFixture(t)
	setRuntimeWildcards(t, fx.schema, []runtime.WildcardRule{
		{},
		{NS: runtime.NSConstraint{Kind: runtime.NSAny}, PC: runtime.PCSkip},
	})
	setRuntimeDFAModels(t, fx.schema, make([]runtime.DFAModel, 2))
	fx.schema.ModelBundle().DFA[1] = runtime.DFAModel{
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
	if match.Kind != StartMatchWildcard || match.Wildcard != 1 {
		t.Fatalf("match = %+v, want wildcard 1", match)
	}
}

func TestModelStateDFAWildcardAmbiguous(t *testing.T) {
	fx := buildModelFixture(t)
	setRuntimeWildcards(t, fx.schema, []runtime.WildcardRule{
		{},
		{NS: runtime.NSConstraint{Kind: runtime.NSAny}, PC: runtime.PCSkip},
		{NS: runtime.NSConstraint{Kind: runtime.NSAny}, PC: runtime.PCSkip},
	})
	setRuntimeDFAModels(t, fx.schema, make([]runtime.DFAModel, 2))
	fx.schema.ModelBundle().DFA[1] = runtime.DFAModel{
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
	setRuntimeDFAModels(t, fx.schema, make([]runtime.DFAModel, 2))
	fx.schema.ModelBundle().DFA[1] = runtime.DFAModel{
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
	setRuntimeDFAModels(t, fx.schema, make([]runtime.DFAModel, 2))
	fx.schema.ModelBundle().DFA[1] = runtime.DFAModel{
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
	setRuntimeDFAModels(t, fx.schema, make([]runtime.DFAModel, 2))
	fx.schema.ModelBundle().DFA[1] = runtime.DFAModel{
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
	setRuntimeNFAModels(t, fx.schema, make([]runtime.NFAModel, 2))
	fx.schema.ModelBundle().NFA[1] = buildNFASequence(fx.symA, fx.symB, fx.elemA, fx.elemB)
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
	if match.Kind != StartMatchElem || match.Elem != fx.elemA {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemA)
	}
	if err := sess.AcceptModel(ref, &state); err == nil {
		t.Fatalf("expected accept failure after single step")
	}

	match, err = sess.StepModel(ref, &state, fx.symB, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel b: %v", err)
	}
	if match.Kind != StartMatchElem || match.Elem != fx.elemB {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemB)
	}
	if err := sess.AcceptModel(ref, &state); err != nil {
		t.Fatalf("AcceptModel: %v", err)
	}
}

func TestSessionModelStatesDoNotAlias(t *testing.T) {
	fx := buildModelFixture(t)
	setRuntimeNFAModels(t, fx.schema, make([]runtime.NFAModel, 2))
	fx.schema.ModelBundle().NFA[1] = buildNFASequence(fx.symA, fx.symB, fx.elemA, fx.elemB)
	setRuntimeAllModels(t, fx.schema, make([]runtime.AllModel, 2))
	fx.schema.ModelBundle().All[1] = runtime.AllModel{
		Members: []runtime.AllMember{
			{Elem: fx.elemA},
			{Elem: fx.elemB},
		},
	}
	sess := NewSession(fx.schema)

	nfaRef := runtime.ModelRef{Kind: runtime.ModelNFA, ID: 1}
	firstNFA, err := sess.InitModelState(nfaRef)
	if err != nil {
		t.Fatalf("InitModelState first NFA: %v", err)
	}
	secondNFA, err := sess.InitModelState(nfaRef)
	if err != nil {
		t.Fatalf("InitModelState second NFA: %v", err)
	}
	firstNFA.NFA[0] = 1
	firstNFA.nfaScratch[0] = 2
	secondNFA.NFA[0] = 3
	secondNFA.nfaScratch[0] = 4
	if firstNFA.NFA[0] != 1 || firstNFA.nfaScratch[0] != 2 {
		t.Fatalf("second NFA state aliased first: first=%v scratch=%v second=%v secondScratch=%v", firstNFA.NFA, firstNFA.nfaScratch, secondNFA.NFA, secondNFA.nfaScratch)
	}

	allRef := runtime.ModelRef{Kind: runtime.ModelAll, ID: 1}
	firstAll, err := sess.InitModelState(allRef)
	if err != nil {
		t.Fatalf("InitModelState first all: %v", err)
	}
	secondAll, err := sess.InitModelState(allRef)
	if err != nil {
		t.Fatalf("InitModelState second all: %v", err)
	}
	firstAll.All[0] = 1
	secondAll.All[0] = 2
	if firstAll.All[0] != 1 {
		t.Fatalf("second all state aliased first: first=%v second=%v", firstAll.All, secondAll.All)
	}
}

func TestSessionValidateNestedNFAAndAllFramesUseIndependentArenaState(t *testing.T) {
	rt := buildNestedNFAAllRuntimeSchema(t)
	plan := runtime.NewSessionPlan(rt)
	if plan.MaxModelWords != 3 {
		t.Fatalf("MaxModelWords = %d, want 3", plan.MaxModelWords)
	}

	sess := NewSession(rt)
	initialCap := cap(sess.buffers.modelWords)
	doc := `<tns:root xmlns:tns="urn:test"><tns:a/><tns:child><tns:d/><tns:c/></tns:child><tns:b/></tns:root>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cap(sess.buffers.modelWords) != initialCap {
		t.Fatalf("modelWords cap = %d, want unchanged %d", cap(sess.buffers.modelWords), initialCap)
	}
}

func TestModelStateNFANullableAcceptsEmpty(t *testing.T) {
	fx := buildModelFixture(t)
	setRuntimeNFAModels(t, fx.schema, make([]runtime.NFAModel, 2))
	fx.schema.ModelBundle().NFA[1] = buildNFANullable(fx.symA, fx.elemA)
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
	setRuntimeNFAModels(t, fx.schema, make([]runtime.NFAModel, 2))
	fx.schema.ModelBundle().NFA[1] = runtime.NFAModel{
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
	setRuntimeAllModels(t, fx.schema, make([]runtime.AllModel, 2))
	setRuntimeAllSubstitutions(t, fx.schema, []runtime.ElemID{fx.elemA, fx.elemC})
	fx.schema.ModelBundle().All[1] = runtime.AllModel{
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
	if match.Kind != StartMatchElem || match.Elem != fx.elemB {
		t.Fatalf("match = %+v, want elem %d", match, fx.elemB)
	}
	match, err = sess.StepModel(ref, &state, fx.symA, fx.ns, []byte("urn:test"))
	if err != nil {
		t.Fatalf("StepModel a: %v", err)
	}
	if match.Kind != StartMatchElem || match.Elem != fx.elemA {
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
	if match.Kind != StartMatchElem || match.Elem != fx.elemC {
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

	fx.schema.ModelBundle().All[1] = runtime.AllModel{
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
	setRuntimeAllModels(t, fx.schema, make([]runtime.AllModel, 2))
	setRuntimeAllSubstitutions(t, fx.schema, []runtime.ElemID{fx.elemA, fx.elemC})
	fx.schema.ModelBundle().All[1] = runtime.AllModel{
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
	setRuntimeAllModels(t, fx.schema, make([]runtime.AllModel, 2))
	setRuntimeAllSubstitutions(t, fx.schema, []runtime.ElemID{
		fx.elemA, fx.elemC,
		fx.elemB, fx.elemC,
	})
	fx.schema.ModelBundle().All[1] = runtime.AllModel{
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

func buildNFASequence3(symA, symB, symC runtime.SymbolID, elemA, elemB, elemC runtime.ElemID) runtime.NFAModel {
	blob := runtime.BitsetBlob{Words: []uint64{1, 4, 2, 4}}
	return runtime.NFAModel{
		Bitsets:   blob,
		Start:     runtime.BitsetRef{Off: 0, Len: 1},
		Accept:    runtime.BitsetRef{Off: 1, Len: 1},
		Nullable:  false,
		FollowOff: 0,
		FollowLen: 3,
		Matchers: []runtime.PosMatcher{
			{Kind: runtime.PosExact, Sym: symA, Elem: elemA},
			{Kind: runtime.PosExact, Sym: symB, Elem: elemB},
			{Kind: runtime.PosExact, Sym: symC, Elem: elemC},
		},
		Follow: []runtime.BitsetRef{
			{Off: 2, Len: 1},
			{Off: 3, Len: 1},
			{},
		},
	}
}

func buildNestedNFAAllRuntimeSchema(t *testing.T) *runtime.Schema {
	t.Helper()

	builder := runtime.NewBuilder()
	ns := mustInternNamespace(t, builder, []byte("urn:test"))
	symRoot := mustInternSymbol(t, builder, ns, []byte("root"))
	symA := mustInternSymbol(t, builder, ns, []byte("a"))
	symChild := mustInternSymbol(t, builder, ns, []byte("child"))
	symC := mustInternSymbol(t, builder, ns, []byte("c"))
	symD := mustInternSymbol(t, builder, ns, []byte("d"))
	symB := mustInternSymbol(t, builder, ns, []byte("b"))

	schema, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	const (
		rootType  runtime.TypeID = 1
		childType runtime.TypeID = 2
		emptyType runtime.TypeID = 3

		rootElem  runtime.ElemID = 1
		aElem     runtime.ElemID = 2
		childElem runtime.ElemID = 3
		cElem     runtime.ElemID = 4
		dElem     runtime.ElemID = 5
		bElem     runtime.ElemID = 6
	)

	setRuntimeRootPolicy(t, schema, runtime.RootStrict)
	setRuntimeTypes(t, schema, make([]runtime.Type, 4))
	schema.TypeTable()[rootType] = runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}}
	schema.TypeTable()[childType] = runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 2}}
	schema.TypeTable()[emptyType] = runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 3}}
	setRuntimeComplexTypes(t, schema, make([]runtime.ComplexType, 4))
	schema.ComplexTypeTable()[1] = runtime.ComplexType{
		Model:   runtime.ModelRef{Kind: runtime.ModelNFA, ID: 1},
		Content: runtime.ContentElementOnly,
	}
	schema.ComplexTypeTable()[2] = runtime.ComplexType{
		Model:   runtime.ModelRef{Kind: runtime.ModelAll, ID: 1},
		Content: runtime.ContentAll,
	}
	schema.ComplexTypeTable()[3] = runtime.ComplexType{Content: runtime.ContentEmpty}

	setRuntimeElements(t, schema, make([]runtime.Element, 7))
	schema.ElementTable()[rootElem] = runtime.Element{Name: symRoot, Type: rootType}
	schema.ElementTable()[aElem] = runtime.Element{Name: symA, Type: emptyType}
	schema.ElementTable()[childElem] = runtime.Element{Name: symChild, Type: childType}
	schema.ElementTable()[cElem] = runtime.Element{Name: symC, Type: emptyType}
	schema.ElementTable()[dElem] = runtime.Element{Name: symD, Type: emptyType}
	schema.ElementTable()[bElem] = runtime.Element{Name: symB, Type: emptyType}
	setRuntimeGlobalElements(t, schema, make([]runtime.ElemID, schema.SymbolCount()+1))
	schema.GlobalElementIDs()[symRoot] = rootElem

	setRuntimeNFAModels(t, schema, make([]runtime.NFAModel, 2))
	schema.ModelBundle().NFA[1] = buildNFASequence3(symA, symChild, symB, aElem, childElem, bElem)
	setRuntimeAllModels(t, schema, make([]runtime.AllModel, 2))
	schema.ModelBundle().All[1] = runtime.AllModel{
		MinOccurs: 1,
		Members: []runtime.AllMember{
			{Elem: cElem},
			{Elem: dElem},
		},
	}
	return schema
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
