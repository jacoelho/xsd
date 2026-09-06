package schema

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func unlimitedCompileContentModelWork(int) error { return nil }

func mustContentModelAnalysis(
	tb testing.TB,
	rt ParticleRuntime,
	work ContentModelWork,
) *ContentModelAnalysis {
	tb.Helper()
	analysis, err := NewContentModelAnalysis(rt, work)
	if err != nil {
		tb.Fatal(err)
	}
	return analysis
}

func checkElementDeclarationsConsistent(rt ElementDeclarationRuntime, model ContentModel) error {
	work := newWorkBudget(contentModelWorkBudget, 10_000)
	checker := newElementDeclarationConsistencyChecker(rt, &work)
	_, err := checker.collectParticles(model.Particles)
	return err
}

func TestCompileContentModelsBuildsWideDFA(t *testing.T) {
	t.Parallel()

	names, rt := compiledModelRuntimeFixture(t, ModelChoice)
	work := newWorkBudget(contentModelWorkBudget, 1_000_000)
	analysis := mustContentModelAnalysis(t, rt, work.spend)
	models, err := CompileContentModels(&names, rt, 1, 32, &work, analysis)
	if err != nil {
		t.Fatalf("CompileContentModels() error = %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("compiled model count = %d, want 1", len(models))
	}
	model := models[0]
	if model.Source != 0 || model.Kind != CompiledModelDFA {
		t.Fatalf("compiled model = {Source:%d Kind:%d}, want source 0 DFA", model.Source, model.Kind)
	}
	if len(model.Rows) == 0 {
		t.Fatal("wide DFA has no rows")
	}
}

func TestCheckContentModelsUPARejectsChoiceOverlap(t *testing.T) {
	t.Parallel()

	names, err := NewNameTable(8, []string{vocab.EmptyNamespaceURI}, []ExpandedName{{Local: "a"}})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, ok := names.LookupQName("", "a")
	if !ok {
		t.Fatal("missing QName for a")
	}
	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[ContentModelID]ContentModel{
			0: {
				Kind:   ModelChoice,
				Occurs: one,
				Particles: []Particle{
					ElementParticle(1, one),
					ElementParticle(2, one),
				},
			},
		},
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
	}
	work := newWorkBudget(contentModelWorkBudget, 1_000_000)
	analysis := mustContentModelAnalysis(t, rt, work.spend)
	err = CheckContentModelsUPA(&names, rt, 1, &work, analysis)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
}

func TestCheckElementDeclarationsConsistentRejectsSameNameDifferentType(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[ContentModelID]ContentModel{
			0: {
				Kind: ModelSequence,
				Particles: []Particle{
					ElementParticle(1, one),
					ModelParticle(1, one),
				},
				Occurs: one,
			},
			1: {
				Kind:      ModelChoice,
				Particles: []Particle{ElementParticle(2, one)},
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
	err := checkElementDeclarationsConsistent(rt, rt.models[0])
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
}

func TestCheckElementDeclarationsConsistentAllowsSameNameSameType(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[ContentModelID]ContentModel{
			0: {
				Kind: ModelSequence,
				Particles: []Particle{
					ElementParticle(1, one),
					ElementParticle(2, one),
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
	if err := checkElementDeclarationsConsistent(rt, rt.models[0]); err != nil {
		t.Fatalf("CheckElementDeclarationsConsistent() error = %v", err)
	}
}

func TestValidateContentRestrictionRejectsElementNillableLoosening(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			ElementParticle(1, one),
			ElementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[ElementID]ParticleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: DeclarationScopeNonGlobal},
			2: {Type: SimpleRef(1), Scope: DeclarationScopeNonGlobal, Nillable: true},
		},
	}
	err := ValidateContentRestriction(rt, 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt, unlimitedCompileContentModelWork))
	if !IsContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want mismatch", err)
	}
}

func TestValidateContentRestrictionAllowsSubstitutionMemberName(t *testing.T) {
	t.Parallel()

	headName := QName{Namespace: EmptyNamespaceID, Local: 1}
	memberName := QName{Namespace: EmptyNamespaceID, Local: 2}
	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			ElementParticle(1, one),
			ElementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: headName,
			2: memberName,
		},
		elementRestrictions: map[ElementID]ParticleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: DeclarationScopeGlobal},
			2: {Type: SimpleRef(2), Scope: DeclarationScopeGlobal, Nillable: true},
		},
		substitutions: map[ElementID]map[QName]ElementID{
			1: {memberName: 2},
		},
	}
	if err := ValidateContentRestriction(rt, 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt, unlimitedCompileContentModelWork)); err != nil {
		t.Fatalf("runtime.ValidateContentRestriction() error = %v", err)
	}
}

func TestValidateContentRestrictionRejectsFixedValueMismatch(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			ElementParticle(1, one),
			ElementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[ElementID]ParticleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: DeclarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "string", "base")},
			2: {Type: SimpleRef(1), Scope: DeclarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "string", "derived")},
		},
	}
	err := ValidateContentRestriction(rt, 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt, unlimitedCompileContentModelWork))
	if !IsContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want mismatch", err)
	}
}

func TestValidateContentRestrictionAllowsFixedCanonicalMatch(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			ElementParticle(1, one),
			ElementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[ElementID]ParticleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: DeclarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "token", "1 2 3")},
			2: {Type: SimpleRef(1), Scope: DeclarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "token", "1   2   3")},
		},
	}
	if err := ValidateContentRestriction(rt, 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt, unlimitedCompileContentModelWork)); err != nil {
		t.Fatalf("runtime.ValidateContentRestriction() error = %v", err)
	}
}

func TestValidateContentRestrictionAllowsFixedValueIdentityMatch(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			ElementParticle(1, one),
			ElementParticle(2, one),
		),
		elementNames: map[ElementID]QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[ElementID]ParticleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: DeclarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "decimal", "5.0")},
			2: {Type: SimpleRef(2), Scope: DeclarationScopeNonGlobal, Fixed: valueConstraintIdentity(t, "integer", "5")},
		},
		simpleDerivations: map[SimpleTypeID]SimpleTypeDerivation{
			1: {Base: NoSimpleType, Variety: SimpleVarietyAtomic},
			2: {Base: 1, Variety: SimpleVarietyAtomic},
		},
	}
	if err := ValidateContentRestriction(rt, 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt, unlimitedCompileContentModelWork)); err != nil {
		t.Fatalf("runtime.ValidateContentRestriction() error = %v", err)
	}
}

func TestValidateContentRestrictionRejectsWildcardOutsideBase(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: 1, Local: 1}
	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[ContentModelID]ContentModel{
			0: {
				Kind:      ModelSequence,
				Particles: []Particle{WildcardParticle(1, one)},
				Occurs:    one,
			},
			1: {
				Kind:      ModelSequence,
				Particles: []Particle{ElementParticle(1, one)},
				Occurs:    one,
			},
		},
		elementNames: map[ElementID]QName{
			1: name,
		},
		elementRestrictions: map[ElementID]ParticleRestrictionElement{
			1: {Type: SimpleRef(1), Scope: DeclarationScopeNonGlobal},
		},
		wildcards: map[WildcardID]Wildcard{
			1: {Mode: WildcardLocal},
		},
	}
	err := ValidateContentRestriction(rt, 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt, unlimitedCompileContentModelWork))
	if !IsContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want mismatch", err)
	}
}

func TestValidateContentRestrictionMissingModelIsInternalInvariant(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[ContentModelID]ContentModel{
			0: {
				Kind:      ModelSequence,
				Particles: []Particle{ElementParticle(1, one)},
				Occurs:    one,
			},
		},
	}
	err := ValidateContentRestriction(rt, 0, 1, unlimitedCompileContentModelWork, mustContentModelAnalysis(t, rt, unlimitedCompileContentModelWork))
	if !IsContentRestrictionInvariant(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want invariant", err)
	}
}

func TestDeterministicRowCapsTransitionGroupBeforeStateLookup(t *testing.T) {
	work := newWorkBudget(contentModelWorkBudget, 1_000_000)
	b := &dfaBuilder{
		c:     &contentModelCompiler{work: &work},
		limit: 1,
		rows: []dfaSourceRow{{
			Edges: []dfaSourceEdge{
				{Particle: Particle{Kind: ParticleElement, Element: 1}, To: 0},
				{Particle: Particle{Kind: ParticleElement, Element: 1}, To: 1},
			},
		}},
	}
	calledStateID := false
	_, err := b.deterministicRow(dfaDeterministicState{Configs: []dfaConfig{{}}}, nil, func(dfaDeterministicState) (uint32, error) {
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
		c:     &contentModelCompiler{work: &work},
		limit: 1,
		rows: []dfaSourceRow{{
			Edges: []dfaSourceEdge{
				{Particle: Particle{Kind: ParticleElement, Element: 1}, To: 0},
				{Particle: Particle{Kind: ParticleElement, Element: 1}, To: 0},
			},
		}},
	}
	var stateConfigs int
	_, err := b.deterministicRow(dfaDeterministicState{Configs: []dfaConfig{{}}}, nil, func(state dfaDeterministicState) (uint32, error) {
		stateConfigs = len(state.Configs)
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
		c: &contentModelCompiler{work: &work},
		rows: []dfaSourceRow{{
			Edges: []dfaSourceEdge{{
				Particle: ElementParticle(1, Occurrence{Min: 1, Max: 1}),
				Guards: []compiledGuard{
					{Slot: 0, N: 1, Kind: compiledGuardLoopMax},
					{Slot: 1, N: 1, Kind: compiledGuardLoopMax},
				},
			}},
		}},
	}
	_, err := b.deterministicRow(
		dfaDeterministicState{Configs: []dfaConfig{{}}},
		nil,
		func(dfaDeterministicState) (uint32, error) { return 0, nil },
	)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}

func TestNormalizeFollowsChargesEntryStorage(t *testing.T) {
	t.Parallel()

	work := newWorkBudget(contentModelWorkBudget, 3)
	b := &dfaBuilder{
		c: &contentModelCompiler{work: &work},
		follow: map[int][]dfaEntry{
			0: {{Pos: 1, Guards: []compiledGuard{{}, {}}}},
		},
	}
	err := b.normalizeFollows()
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}

func TestNormalizeSingleParticleModelCollapsesContiguousOccurrenceRange(t *testing.T) {
	t.Parallel()

	model := ContentModel{
		Kind:   ModelChoice,
		Occurs: Occurrence{Min: 0, Max: 40},
		Particles: []Particle{ElementParticle(1, Occurrence{
			Min: 0,
			Max: 100,
		})},
	}
	got := normalizeSingleParticleModel(model)
	if got.Kind != ModelSequence || !got.Occurs.IsExactlyOne() || len(got.Particles) != 1 {
		t.Fatalf("normalized model = %+v", got)
	}
	if want := (Occurrence{Min: 0, Max: 4000}); got.Particles[0].Occurs != want {
		t.Fatalf("normalized occurrence = %+v, want %+v", got.Particles[0].Occurs, want)
	}
}

func TestNormalizeSingleParticleModelPreservesDiscontinuousOccurrenceRange(t *testing.T) {
	t.Parallel()

	model := ContentModel{
		Kind:      ModelSequence,
		Occurs:    Occurrence{Min: 0, Max: 2},
		Particles: []Particle{ElementParticle(1, Occurrence{Min: 2, Max: 3})},
	}
	got := normalizeSingleParticleModel(model)
	if got.Kind != model.Kind || got.Occurs != model.Occurs || got.Particles[0].Occurs != model.Particles[0].Occurs {
		t.Fatalf("unsafe normalization changed model: got %+v want %+v", got, model)
	}
}

func TestNormalizeSingleParticleModelPreservesOverflowingOccurrenceProduct(t *testing.T) {
	t.Parallel()

	model := ContentModel{
		Kind:      ModelSequence,
		Occurs:    Occurrence{Min: 2, Max: 2},
		Particles: []Particle{ElementParticle(1, Occurrence{Min: 3_000_000_000, Max: 3_000_000_000})},
	}
	got := normalizeSingleParticleModel(model)
	if got.Kind != model.Kind || got.Occurs != model.Occurs || got.Particles[0].Occurs != model.Particles[0].Occurs {
		t.Fatalf("overflowing normalization changed model: got %+v want %+v", got, model)
	}
}

func TestNormalizeSingleParticleModelPreservesChoiceLimits(t *testing.T) {
	t.Parallel()

	model := ContentModel{
		Kind:         ModelSequence,
		Occurs:       Occurrence{Min: 1, Max: 1},
		Particles:    []Particle{ElementParticle(1, Occurrence{Min: 0, Unbounded: true})},
		ChoiceLimits: []uint32{0},
	}
	got := normalizeSingleParticleModel(model)
	if got.Kind != model.Kind || got.Occurs != model.Occurs ||
		!slices.Equal(got.Particles, model.Particles) || !slices.Equal(got.ChoiceLimits, model.ChoiceLimits) {
		t.Fatalf("choice-limited normalization changed model: got %+v want %+v", got, model)
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

type compiledModelRuntimeStub struct {
	models              map[ContentModelID]ContentModel
	elementNames        map[ElementID]QName
	elementTypes        map[ElementID]TypeID
	elementRestrictions map[ElementID]ParticleRestrictionElement
	wildcards           map[WildcardID]Wildcard
	substitutions       map[ElementID]map[QName]ElementID
	simpleDerivations   map[SimpleTypeID]SimpleTypeDerivation
	complexDerivations  map[ComplexTypeID]ComplexTypeDerivation
	anyType             ComplexTypeID
}

func restrictionModels(base, derived Particle) map[ContentModelID]ContentModel {
	one := Occurrence{Min: 1, Max: 1}
	return map[ContentModelID]ContentModel{
		0: {
			Kind:      ModelSequence,
			Particles: []Particle{base},
			Occurs:    one,
		},
		1: {
			Kind:      ModelSequence,
			Particles: []Particle{derived},
			Occurs:    one,
		},
	}
}

func compiledModelRuntimeFixture(t *testing.T, kind ModelKind) (NameTable, compiledModelRuntimeStub) {
	t.Helper()

	var required []ExpandedName
	elementNames := make(map[ElementID]QName)
	particles := make([]Particle, 8)
	for i := range particles {
		local := string(rune('a' + i))
		required = append(required, ExpandedName{Local: local})
	}
	names, err := NewNameTable(64, []string{vocab.EmptyNamespaceURI}, required)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	one := Occurrence{Min: 1, Max: 1}
	for i := range particles {
		local := string(rune('a' + i))
		name, ok := names.LookupQName("", local)
		if !ok {
			t.Fatalf("QName %q missing", local)
		}
		id := ElementID(i + 1)
		elementNames[id] = name
		particles[i] = ElementParticle(id, one)
	}
	rt := compiledModelRuntimeStub{
		models: map[ContentModelID]ContentModel{
			0: {Kind: kind, Occurs: one, Particles: particles},
		},
		elementNames: elementNames,
		elementTypes: make(map[ElementID]TypeID, len(elementNames)),
	}
	for id := range elementNames {
		rt.elementTypes[id] = SimpleRef(SimpleTypeID(id))
	}
	return names, rt
}

func (s compiledModelRuntimeStub) ContentModel(id ContentModelID) (ContentModel, bool) {
	model, ok := s.models[id]
	return model, ok
}

func (s compiledModelRuntimeStub) ElementName(id ElementID) (QName, bool) {
	name, ok := s.elementNames[id]
	return name, ok
}

func (s compiledModelRuntimeStub) ElementType(id ElementID) (TypeID, bool) {
	if decl, ok := s.elementRestrictions[id]; ok {
		return decl.Type, true
	}
	typ, ok := s.elementTypes[id]
	return typ, ok
}

func (s compiledModelRuntimeStub) ElementRestriction(id ElementID) (ParticleRestrictionElement, bool) {
	if decl, ok := s.elementRestrictions[id]; ok {
		return decl, true
	}
	typ, ok := s.elementTypes[id]
	if !ok {
		return ParticleRestrictionElement{}, false
	}
	return ParticleRestrictionElement{Type: typ, Scope: DeclarationScopeNonGlobal}, true
}

func (s compiledModelRuntimeStub) Wildcard(id WildcardID) (Wildcard, bool) {
	wildcard, ok := s.wildcards[id]
	return wildcard, ok
}

func (s compiledModelRuntimeStub) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	members := s.substitutions[id]
	if members == nil {
		return
	}
	for _, member := range members {
		if !fn(member) {
			return
		}
	}
}

func (s compiledModelRuntimeStub) HasSubstitutionMembers(id ElementID) bool {
	return len(s.substitutions[id]) != 0
}

func (s compiledModelRuntimeStub) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	members := s.substitutions[id]
	if members == nil {
		return NoElement, false
	}
	member, ok := members[name]
	return member, ok
}

func (s compiledModelRuntimeStub) ForEachSubstitutionEntry(id ElementID, fn func(QName, ElementID) bool) {
	for name, member := range s.substitutions[id] {
		if !fn(name, member) {
			return
		}
	}
}

func (s compiledModelRuntimeStub) AnyTypeID() ComplexTypeID {
	return s.anyType
}

func (s compiledModelRuntimeStub) ComplexTypeCount() int {
	var count int
	for id := range s.complexDerivations {
		count = max(count, int(id)+1)
	}
	if s.anyType != 0 {
		count = max(count, int(s.anyType)+1)
	}
	return count
}

func (s compiledModelRuntimeStub) SimpleTypeCount() int {
	return len(s.simpleDerivations)
}

func (s compiledModelRuntimeStub) SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	derivation, ok := s.simpleDerivations[id]
	return derivation, ok
}

func (s compiledModelRuntimeStub) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	derivation, ok := s.complexDerivations[id]
	return derivation, ok
}
