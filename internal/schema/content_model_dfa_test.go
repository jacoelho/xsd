package schema

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func unlimitedCompileContentModelWork(int) error { return nil }

func mustContentModelAnalysis(
	tb testing.TB,
	rt *schemaBuild,
	work contentModelWork,
) *contentModelAnalysis {
	tb.Helper()
	analysis, err := newContentModelAnalysis(rt, work)
	if err != nil {
		tb.Fatal(err)
	}
	return analysis
}

func checkElementDeclarationsConsistent(rt *schemaBuild, model contentModelNode) error {
	work := newWorkBudget(contentModelWorkBudget, 10_000)
	checker := newElementDeclarationConsistencyChecker(rt, work.spend)
	_, err := checker.collectParticles(model.Particles)
	return err
}

func TestCompileContentModelsBuildsWideDFA(t *testing.T) {
	t.Parallel()

	rt := compiledModelRuntimeFixture(t, modelChoice)
	work := newWorkBudget(contentModelWorkBudget, 1_000_000)
	analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
	cc := newContentModelCompiler(rt.build(), 32, &work, testDependencyWork(), analysis)
	model, err := cc.compileRootContentModel(0)
	if err != nil {
		t.Fatalf("compileRootContentModel() error = %v", err)
	}
	if model.Kind != compiledModelDFA {
		t.Fatalf("compiled model kind = %d, want DFA", model.Kind)
	}
	if len(model.Rows) == 0 {
		t.Fatal("wide DFA has no rows")
	}
}

func TestCompileContentModelExecutesNestedGroupsAsTheirFlatForm(t *testing.T) {
	t.Parallel()

	for _, kind := range []modelKind{modelSequence, modelChoice} {
		rt := compiledModelRuntimeFixture(t, kind)
		one := occurrence{Min: 1, Max: 1}
		flat := rt.models[0].Particles
		const (
			innerID  contentModelID = 1
			emptyID  contentModelID = 2
			nestedID contentModelID = 3
		)
		rt.models[innerID] = contentModelNode{Kind: kind, Occurs: one, Particles: flat[:3]}
		rt.models[emptyID] = contentModelNode{Kind: modelSequence, Occurs: one}
		nested := []contentParticle{modelParticle(innerID, one)}
		if kind == modelSequence {
			nested = append(nested, modelParticle(emptyID, one))
		}
		rt.models[nestedID] = contentModelNode{Kind: kind, Occurs: one, Particles: append(nested, flat[3:]...)}

		compile := func(id contentModelID) compiledModel {
			work := newWorkBudget(contentModelWorkBudget, 1_000_000)
			analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
			cc := newContentModelCompiler(rt.build(), 32, &work, testDependencyWork(), analysis)
			model, err := cc.compileRootContentModel(id)
			if err != nil {
				t.Fatalf("compileRootContentModel(%d) error = %v", id, err)
			}
			return model
		}
		if got, want := compile(nestedID), compile(0); !reflect.DeepEqual(got, want) {
			t.Fatalf("kind %d: nested model compiled to %+v, want the flat model's %+v", kind, got, want)
		}
	}
}

func TestCompileContentModelExecutesReducedLaxAnyContentWithoutDFA(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	many := occurrence{Unbounded: true}
	const (
		lax    WildcardID = 0
		strict WildcardID = 1
	)
	group := func(kind modelKind, occurs occurrence, p contentParticle) contentModelNode {
		return contentModelNode{Kind: kind, Occurs: occurs, Particles: []contentParticle{p}}
	}
	for _, test := range []struct {
		name   string
		models map[contentModelID]contentModelNode
		want   compiledModelKind
	}{
		{name: "sequence of a choice of the wildcard", want: compiledModelAny, models: map[contentModelID]contentModelNode{
			0: group(modelSequence, one, modelParticle(1, one)),
			1: group(modelChoice, one, wildcardParticle(lax, many)),
		}},
		{name: "optional sequence of the wildcard", want: compiledModelAny, models: map[contentModelID]contentModelNode{
			0: group(modelChoice, one, modelParticle(1, occurrence{Max: 1})),
			1: group(modelSequence, one, wildcardParticle(lax, many)),
		}},
		{name: "sequence of a choice of a required wildcard", want: compiledModelDFA, models: map[contentModelID]contentModelNode{
			0: group(modelSequence, one, modelParticle(1, one)),
			1: group(modelChoice, one, wildcardParticle(lax, occurrence{Min: 1, Unbounded: true})),
		}},
		{name: "sequence of a choice of a strict wildcard", want: compiledModelDFA, models: map[contentModelID]contentModelNode{
			0: group(modelSequence, one, modelParticle(1, one)),
			1: group(modelChoice, one, wildcardParticle(strict, many)),
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rt := compiledModelFixture{
				models: test.models,
				wildcards: map[WildcardID]wildcardRuntime{
					lax:    {Mode: WildcardAny, Process: ProcessLax},
					strict: {Mode: WildcardAny, Process: ProcessStrict},
				},
			}
			work := newWorkBudget(contentModelWorkBudget, 1_000_000)
			analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
			cc := newContentModelCompiler(rt.build(), 32, &work, testDependencyWork(), analysis)
			model, err := cc.compileRootContentModel(0)
			if err != nil {
				t.Fatalf("compileRootContentModel() error = %v", err)
			}
			if model.Kind != test.want {
				t.Fatalf("compiled model kind = %d, want %d", model.Kind, test.want)
			}
		})
	}
}

func TestCheckContentModelsUPARejectsChoiceOverlap(t *testing.T) {
	t.Parallel()

	names, err := newNameTable(8, []string{vocab.EmptyNamespaceURI}, []ExpandedName{{Local: "a"}})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, ok := names.LookupQName("", "a")
	if !ok {
		t.Fatal("missing QName for a")
	}
	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: map[contentModelID]contentModelNode{
			0: {
				Kind:   modelChoice,
				Occurs: one,
				Particles: []contentParticle{
					elementParticle(1, one),
					elementParticle(2, one),
				},
			},
		},
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
	}
	work := newWorkBudget(contentModelWorkBudget, 1_000_000)
	analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
	err = newContentModelUPACheck(rt.build(), 1, &work, testDependencyWork(), analysis).checkModel(0, namedComplexTypeLabel(name))
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
}

func TestCheckContentModelsUPARejectsCorruptRuntimeSplitInputs(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	twice := occurrence{Min: 1, Max: 2}
	for _, tt := range []struct {
		name    string
		models  map[contentModelID]contentModelNode
		visited int
	}{
		{
			name:    "particle names a missing model",
			models:  map[contentModelID]contentModelNode{0: {Kind: modelSequence, Occurs: one, Particles: []contentParticle{modelParticle(5, twice)}}},
			visited: 1,
		},
		{
			name: "model outside the visit table",
			models: map[contentModelID]contentModelNode{
				0: {Kind: modelSequence, Occurs: one, Particles: []contentParticle{modelParticle(1, twice)}},
				1: {Kind: modelChoice, Occurs: one, Particles: []contentParticle{elementParticle(0, one)}},
			},
			visited: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rt := compiledModelFixture{models: tt.models}
			work := newWorkBudget(contentModelWorkBudget, 1_000_000)
			analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
			err := newContentModelUPACheck(rt.build(), tt.visited, &work, testDependencyWork(), analysis).checkModel(0, namedComplexTypeLabel(QName{}))
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestCheckElementDeclarationsConsistentRejectsSameNameDifferentType(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: emptyNamespaceID, Local: 1}
	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: map[contentModelID]contentModelNode{
			0: {
				Kind: modelSequence,
				Particles: []contentParticle{
					elementParticle(1, one),
					modelParticle(1, one),
				},
				Occurs: one,
			},
			1: {
				Kind:      modelChoice,
				Particles: []contentParticle{elementParticle(2, one)},
				Occurs:    one,
			},
		},
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementTypes: map[ElementID]TypeID{
			1: SimpleRef(1),
			2: SimpleRef(2),
		},
	}
	err := checkElementDeclarationsConsistent(rt.build(), rt.models[0])
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
}

func TestCheckElementDeclarationsConsistentAllowsSameNameSameType(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: emptyNamespaceID, Local: 1}
	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: map[contentModelID]contentModelNode{
			0: {
				Kind: modelSequence,
				Particles: []contentParticle{
					elementParticle(1, one),
					elementParticle(2, one),
				},
				Occurs: one,
			},
		},
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementTypes: map[ElementID]TypeID{
			1: ComplexRef(1),
			2: ComplexRef(1),
		},
	}
	build := rt.build()
	typeName := QName{Namespace: emptyNamespaceID, Local: 2}
	build.ComplexTypes = make([]complexTypeMetadata, 2)
	build.ComplexTypes[1] = complexTypeMetadata{Name: typeName, Scope: declarationScopeGlobal}
	build.GlobalTypes = map[QName]TypeID{typeName: ComplexRef(1)}
	if err := checkElementDeclarationsConsistent(build, rt.models[0]); err != nil {
		t.Fatalf("CheckElementDeclarationsConsistent() error = %v", err)
	}
}

func TestValidateContentRestrictionRejectsElementNillableLoosening(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: emptyNamespaceID, Local: 1}
	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: restrictionModels(
			elementParticle(1, one),
			elementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[ElementID]particleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: declarationScopeNonGlobal},
			2: {Type: SimpleRef(1), Scope: declarationScopeNonGlobal, Nillable: true},
		},
	}
	err := validateContentRestriction(rt.build(), 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt.build(), unlimitedCompileContentModelWork))
	if !isContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want mismatch", err)
	}
}

func TestValidateContentRestrictionAllowsSubstitutionMemberName(t *testing.T) {
	t.Parallel()

	headName := QName{Namespace: emptyNamespaceID, Local: 1}
	memberName := QName{Namespace: emptyNamespaceID, Local: 2}
	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: restrictionModels(
			elementParticle(1, one),
			elementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: headName,
			2: memberName,
		},
		elementRestrictions: map[ElementID]particleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: declarationScopeGlobal},
			2: {Type: SimpleRef(2), Scope: declarationScopeGlobal, Nillable: true},
		},
		substitutions: map[ElementID]map[QName]ElementID{
			1: {memberName: 2},
		},
	}
	if err := validateContentRestriction(rt.build(), 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt.build(), unlimitedCompileContentModelWork)); err != nil {
		t.Fatalf("runtime.ValidateContentRestriction() error = %v", err)
	}
}

func TestValidateContentRestrictionRejectsFixedValueMismatch(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: emptyNamespaceID, Local: 1}
	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: restrictionModels(
			elementParticle(1, one),
			elementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[ElementID]particleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: declarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "string", "base")},
			2: {Type: SimpleRef(1), Scope: declarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "string", "derived")},
		},
	}
	err := validateContentRestriction(rt.build(), 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt.build(), unlimitedCompileContentModelWork))
	if !isContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want mismatch", err)
	}
}

func TestValidateContentRestrictionAllowsFixedCanonicalMatch(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: emptyNamespaceID, Local: 1}
	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: restrictionModels(
			elementParticle(1, one),
			elementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[ElementID]particleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: declarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "token", "1 2 3")},
			2: {Type: SimpleRef(1), Scope: declarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "token", "1   2   3")},
		},
	}
	if err := validateContentRestriction(rt.build(), 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt.build(), unlimitedCompileContentModelWork)); err != nil {
		t.Fatalf("runtime.ValidateContentRestriction() error = %v", err)
	}
}

func TestValidateContentRestrictionAllowsFixedValueIdentityMatch(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: emptyNamespaceID, Local: 1}
	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: restrictionModels(
			elementParticle(1, one),
			elementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[ElementID]particleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: declarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "decimal", "5.0")},
			2: {Type: SimpleRef(2), Scope: declarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "integer", "5")},
		},
		simpleDerivations: map[SimpleTypeID]simpleTypeDerivation{
			1: {Base: NoSimpleType, Variety: valuepkg.Atomic},
			2: {Base: 1, Variety: valuepkg.Atomic},
		},
	}
	if err := validateContentRestriction(rt.build(), 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt.build(), unlimitedCompileContentModelWork)); err != nil {
		t.Fatalf("runtime.ValidateContentRestriction() error = %v", err)
	}
}

func TestValidateContentRestrictionRejectsWildcardOutsideBase(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: 1, Local: 1}
	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: map[contentModelID]contentModelNode{
			0: {
				Kind:      modelSequence,
				Particles: []contentParticle{wildcardParticle(1, one)},
				Occurs:    one,
			},
			1: {
				Kind:      modelSequence,
				Particles: []contentParticle{elementParticle(1, one)},
				Occurs:    one,
			},
		},
		elementNames: map[ElementID]QName{
			1: name,
		},
		elementRestrictions: map[ElementID]particleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: declarationScopeNonGlobal},
		},
		wildcards: map[WildcardID]wildcardRuntime{
			1: {Mode: WildcardLocal},
		},
	}
	err := validateContentRestriction(rt.build(), 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt.build(), unlimitedCompileContentModelWork))
	if !isContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want mismatch", err)
	}
}

func TestValidateContentRestrictionMissingModelIsInternalInvariant(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{
		models: map[contentModelID]contentModelNode{
			0: {
				Kind:      modelSequence,
				Particles: []contentParticle{elementParticle(1, one)},
				Occurs:    one,
			},
		},
	}
	err := validateContentRestriction(rt.build(), 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt.build(), unlimitedCompileContentModelWork))
	if !isBuildDefect(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want invariant", err)
	}
}

func TestDeterministicRowCapsTransitionGroupBeforeStateLookup(t *testing.T) {
	work := newWorkBudget(contentModelWorkBudget, 1_000_000)
	b := &dfaBuilder{
		c:     &contentModelCompiler{work: &work, retained: testDependencyWork()},
		limit: 1,
		rows: []dfaSourceRow{{
			Edges: []dfaSourceEdge{
				{Particle: contentParticle{Kind: particleElement, Element: 1}, To: 0},
				{Particle: contentParticle{Kind: particleElement, Element: 1}, To: 1},
			},
		}},
	}
	calledStateID := false
	_, err := mustDeterministicModelCompilation(t, b).deterministicRow([]dfaConfig{{}}, func([]dfaConfig) (uint32, error) {
		calledStateID = true
		return 0, nil
	})
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	if calledStateID {
		t.Fatal("deterministicRow called stateID after transition group exceeded limit")
	}
}

func TestDeterministicRowDeduplicatesTransitionGroupBeforeLimit(t *testing.T) {
	work := newWorkBudget(contentModelWorkBudget, 1_000_000)
	b := &dfaBuilder{
		c:     &contentModelCompiler{work: &work, retained: testDependencyWork()},
		limit: 1,
		rows: []dfaSourceRow{{
			Edges: []dfaSourceEdge{
				{Particle: contentParticle{Kind: particleElement, Element: 1}, To: 0},
				{Particle: contentParticle{Kind: particleElement, Element: 1}, To: 0},
			},
		}},
	}
	var stateConfigs int
	_, err := mustDeterministicModelCompilation(t, b).deterministicRow([]dfaConfig{{}}, func(configs []dfaConfig) (uint32, error) {
		stateConfigs = len(configs)
		return 0, nil
	})
	if err != nil {
		t.Fatalf("deterministicRow() error = %v", err)
	}
	if stateConfigs != 1 {
		t.Fatalf("state configs = %d, want compacted duplicate", stateConfigs)
	}
}

func TestDeterministicRowChargesRejectedGuards(t *testing.T) {
	t.Parallel()

	work := newWorkBudget(contentModelWorkBudget, 2)
	b := &dfaBuilder{
		c: &contentModelCompiler{work: &work, retained: testDependencyWork()},
		rows: []dfaSourceRow{{
			Edges: []dfaSourceEdge{{
				Particle: elementParticle(1, occurrence{Min: 1, Max: 1}),
				Guards: []compiledGuard{
					{Slot: 0, N: 1, Kind: compiledGuardLoopMax},
					{Slot: 1, N: 1, Kind: compiledGuardLoopMax},
				},
			}},
		}},
	}
	_, err := mustDeterministicModelCompilation(t, b).deterministicRow(
		[]dfaConfig{{}},
		func([]dfaConfig) (uint32, error) { return 0, nil },
	)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}

func mustDeterministicModelCompilation(t *testing.T, b *dfaBuilder) *deterministicModelCompilation {
	t.Helper()
	c, err := newDeterministicModelCompilation(b, nil)
	if err != nil {
		t.Fatalf("newDeterministicModelCompilation() error = %v", err)
	}
	return c
}

func TestNormalizeFollowsChargesFollowEntryGuards(t *testing.T) {
	t.Parallel()

	follow := newFollowTable()
	follow.addPosition()
	follow.addPosition()
	set, err := follow.addSet([]int{1})
	if err != nil {
		t.Fatal(err)
	}
	guards := []compiledGuard{{Slot: 0, N: 2, Kind: compiledGuardLoopMax}, {Slot: 1, N: 2, Kind: compiledGuardLoopMax}}
	if err = follow.add(0, guards, nil, set); err != nil {
		t.Fatal(err)
	}
	work := newWorkBudget(contentModelWorkBudget, 3)
	b := &dfaBuilder{c: &contentModelCompiler{work: &work, retained: testDependencyWork()}, follow: follow}
	err = b.normalizeFollows()
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}

func TestFollowStateChargesEachEntryWithItsGuardsAndActions(t *testing.T) {
	t.Parallel()

	guards := []compiledGuard{{Slot: 0, N: 2, Kind: compiledGuardLoopMax}, {Slot: 1, N: 3, Kind: compiledGuardExitMin}}
	actions := []compiledAction{{Slot: 0, Kind: compiledActionInc}}
	const twoEntriesWithTwoGuardsAndOneActionEach = 2 + 1 + 2*(2+1)
	for _, tc := range []struct {
		budget int
		fits   bool
	}{
		{budget: twoEntriesWithTwoGuardsAndOneActionEach, fits: true},
		{budget: twoEntriesWithTwoGuardsAndOneActionEach - 1, fits: false},
	} {
		follow := newFollowTable()
		for range 3 {
			follow.addPosition()
		}
		set, err := follow.addSet([]int{1, 2})
		if err != nil {
			t.Fatal(err)
		}
		if err = follow.add(0, guards, actions, set); err != nil {
			t.Fatal(err)
		}
		if err = follow.chargeNormalization(func(int) error { return nil }); err != nil {
			t.Fatal(err)
		}
		work := newWorkBudget(contentModelWorkBudget, tc.budget)
		b := &dfaBuilder{c: &contentModelCompiler{work: &work, retained: testDependencyWork()}, follow: follow, states: make(map[string]uint32), limit: 1}
		_, err = b.followStateID(0)
		if tc.fits {
			if err != nil {
				t.Fatalf("followStateID() with budget %d error = %v, want nil", tc.budget, err)
			}
			continue
		}
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	}
}

// followTableOf returns a follow table in which each position of follows is
// followed by exactly its positions, without guards or actions.
func followTableOf(follows map[int][]int) followTable {
	t := newFollowTable()
	for pos, positions := range follows {
		for len(t.heads) <= pos {
			t.addPosition()
		}
		set, err := t.addSet(positions)
		if err != nil {
			panic(err)
		}
		if err := t.add(pos, nil, nil, set); err != nil {
			panic(err)
		}
	}
	return t
}

func TestNormalizeSingleParticleModelCollapsesContiguousOccurrenceRange(t *testing.T) {
	t.Parallel()

	model := contentModelNode{
		Kind:   modelChoice,
		Occurs: occurrence{Min: 0, Max: 40},
		Particles: []contentParticle{elementParticle(1, occurrence{
			Min: 0,
			Max: 100,
		})},
	}
	got := normalizeSingleParticleModel(model)
	if got.Kind != modelSequence || !got.Occurs.IsExactlyOne() || len(got.Particles) != 1 {
		t.Fatalf("normalized model = %+v", got)
	}
	if want := (occurrence{Min: 0, Max: 4000}); got.Particles[0].Occurs != want {
		t.Fatalf("normalized occurrence = %+v, want %+v", got.Particles[0].Occurs, want)
	}
}

func TestNormalizeSingleParticleModelPreservesDiscontinuousOccurrenceRange(t *testing.T) {
	t.Parallel()

	model := contentModelNode{
		Kind:      modelSequence,
		Occurs:    occurrence{Min: 0, Max: 2},
		Particles: []contentParticle{elementParticle(1, occurrence{Min: 2, Max: 3})},
	}
	got := normalizeSingleParticleModel(model)
	if got.Kind != model.Kind || got.Occurs != model.Occurs || got.Particles[0].Occurs != model.Particles[0].Occurs {
		t.Fatalf("unsafe normalization changed model: got %+v want %+v", got, model)
	}
}

func TestNormalizeSingleParticleModelPreservesOverflowingOccurrenceProduct(t *testing.T) {
	t.Parallel()

	model := contentModelNode{
		Kind:      modelSequence,
		Occurs:    occurrence{Min: 2, Max: 2},
		Particles: []contentParticle{elementParticle(1, occurrence{Min: 3_000_000_000, Max: 3_000_000_000})},
	}
	got := normalizeSingleParticleModel(model)
	if got.Kind != model.Kind || got.Occurs != model.Occurs || got.Particles[0].Occurs != model.Particles[0].Occurs {
		t.Fatalf("overflowing normalization changed model: got %+v want %+v", got, model)
	}
}

func expectDiagnostic(t *testing.T, err error, category xsderrors.Category, code xsderrors.Code) {
	t.Helper()
	var diag *xsderrors.Error
	if !errors.As(err, &diag) {
		t.Fatalf("error = %v, want xsderrors.Error", err)
	}
	if diag.Category() != category || diag.Code() != code {
		t.Fatalf("diagnostic = (%s, %s), want (%s, %s)", diag.Category(), diag.Code(), category, code)
	}
}

type compiledModelFixture struct {
	models              map[contentModelID]contentModelNode
	elementNames        map[ElementID]QName
	elementTypes        map[ElementID]TypeID
	elementRestrictions map[ElementID]particleRestrictionElement
	wildcards           map[WildcardID]wildcardRuntime
	substitutions       map[ElementID]map[QName]ElementID
	simpleDerivations   map[SimpleTypeID]simpleTypeDerivation
	complexDerivations  map[ComplexTypeID]complexTypeDerivation
	anyType             ComplexTypeID
}

func restrictionModels(base, derived contentParticle) map[contentModelID]contentModelNode {
	one := occurrence{Min: 1, Max: 1}
	return map[contentModelID]contentModelNode{
		0: {
			Kind:      modelSequence,
			Particles: []contentParticle{base},
			Occurs:    one,
		},
		1: {
			Kind:      modelSequence,
			Particles: []contentParticle{derived},
			Occurs:    one,
		},
	}
}

func compiledModelRuntimeFixture(t *testing.T, kind modelKind) compiledModelFixture {
	t.Helper()

	var required []ExpandedName
	elementNames := make(map[ElementID]QName)
	particles := make([]contentParticle, 8)
	for i := range particles {
		local := string(rune('a' + i))
		required = append(required, ExpandedName{Local: local})
	}
	names, err := newNameTable(64, []string{vocab.EmptyNamespaceURI}, required)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	one := occurrence{Min: 1, Max: 1}
	for i := range particles {
		local := string(rune('a' + i))
		name, ok := names.LookupQName("", local)
		if !ok {
			t.Fatalf("QName %q missing", local)
		}
		id := ElementID(i + 1)
		elementNames[id] = name
		particles[i] = elementParticle(id, one)
	}
	rt := compiledModelFixture{
		models: map[contentModelID]contentModelNode{
			0: {Kind: kind, Occurs: one, Particles: particles},
		},
		elementNames: elementNames,
		elementTypes: make(map[ElementID]TypeID, len(elementNames)),
	}
	for id := range elementNames {
		rt.elementTypes[id] = SimpleRef(SimpleTypeID(id))
	}
	return rt
}

func (f compiledModelFixture) build() *schemaBuild {
	b := &schemaBuild{Models: fixtureMapSlice(f.models), Wildcards: fixtureMapSlice(f.wildcards), AnyType: f.anyType}
	for i := range b.Models {
		if !b.Models[i].NonUnitParticle {
			b.Models[i].NonUnitParticle = slices.ContainsFunc(b.Models[i].Particles, nonUnitParticle)
		}
	}
	names := fixtureMapSlice(f.elementNames)
	rs := fixtureMapSlice(f.elementRestrictions)
	b.Elements = fixtureElements(names, rs)
	for id, typ := range f.elementTypes {
		if !validUint32Index(uint32(id), len(b.Elements)) {
			b.Elements = append(b.Elements, make([]elementDecl, int(id)+1-len(b.Elements))...)
		}
		b.Elements[id].Type = typ
		b.Elements[id].Scope = declarationScopeNonGlobal
	}
	b.Substitutions = fixtureSubstitutions(b.Elements, nil, f.substitutions)
	b.SimpleTypes = fixtureSimpleTypes(fixtureMapSlice(f.simpleDerivations))
	for i := range b.SimpleTypes {
		if _, ok := f.simpleDerivations[SimpleTypeID(i)]; !ok {
			b.SimpleTypes[i].Missing = true
		}
	}
	b.ComplexTypes = fixtureComplexTypes(fixtureMapSlice(f.complexDerivations))
	return b
}
