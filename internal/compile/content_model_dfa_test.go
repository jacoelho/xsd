package compile

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompileContentModelsBuildsIndexedRows(t *testing.T) {
	t.Parallel()

	names, rt := compiledModelRuntimeFixture(t, runtime.ModelChoice)
	models, err := CompileContentModels(&names, rt, 1, 32)
	if err != nil {
		t.Fatalf("CompileContentModels() error = %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("compiled model count = %d, want 1", len(models))
	}
	model := models[0]
	if model.Source != 0 || model.Kind != runtime.CompiledModelDFA {
		t.Fatalf("compiled model = {Source:%d Kind:%d}, want source 0 DFA", model.Source, model.Kind)
	}
	if len(model.Rows) == 0 || !model.Rows[0].Index.IsEnabled() {
		t.Fatal("wide compiled row was not indexed")
	}
}

func TestValidateCompiledModelDerivedRejectsDrift(t *testing.T) {
	t.Parallel()

	names, rt := compiledModelRuntimeFixture(t, runtime.ModelChoice)
	models, err := CompileContentModels(&names, rt, 1, 32)
	if err != nil {
		t.Fatalf("CompileContentModels() error = %v", err)
	}
	model := models[0]
	model.Rows[0].Edges[0].To = model.Rows[0].Edges[1].To
	err = ValidateCompiledModelDerived(&names, rt, 0, model)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestCheckContentModelsUPARejectsChoiceOverlap(t *testing.T) {
	t.Parallel()

	names, err := runtime.NewNameTable(8, []string{runtime.EmptyNamespaceURI}, []runtime.ExpandedName{{Local: "a"}})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, ok := names.LookupQName("", "a")
	if !ok {
		t.Fatal("missing QName for a")
	}
	one := runtime.Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[runtime.ContentModelID]runtime.ContentModel{
			0: {
				Kind:   runtime.ModelChoice,
				Occurs: one,
				Particles: []runtime.Particle{
					runtime.ElementParticle(1, one),
					runtime.ElementParticle(2, one),
				},
			},
		},
		elementNames: map[runtime.ElementID]runtime.QName{
			1: name,
			2: name,
		},
	}
	err = CheckContentModelsUPA(&names, rt, 1)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
}

func TestCheckElementDeclarationsConsistentRejectsSameNameDifferentType(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	one := runtime.Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[runtime.ContentModelID]runtime.ContentModel{
			0: {
				Kind: runtime.ModelSequence,
				Particles: []runtime.Particle{
					runtime.ElementParticle(1, one),
					runtime.ModelParticle(1, one),
				},
				Occurs: one,
			},
			1: {
				Kind:      runtime.ModelChoice,
				Particles: []runtime.Particle{runtime.ElementParticle(2, one)},
				Occurs:    one,
			},
		},
		elementNames: map[runtime.ElementID]runtime.QName{
			1: name,
			2: name,
		},
		elementTypes: map[runtime.ElementID]runtime.TypeID{
			1: runtime.SimpleRef(1),
			2: runtime.SimpleRef(2),
		},
	}
	err := CheckElementDeclarationsConsistent(rt, rt.models[0])
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
}

func TestCheckElementDeclarationsConsistentAllowsSameNameSameType(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	one := runtime.Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[runtime.ContentModelID]runtime.ContentModel{
			0: {
				Kind: runtime.ModelSequence,
				Particles: []runtime.Particle{
					runtime.ElementParticle(1, one),
					runtime.ElementParticle(2, one),
				},
				Occurs: one,
			},
		},
		elementNames: map[runtime.ElementID]runtime.QName{
			1: name,
			2: name,
		},
		elementTypes: map[runtime.ElementID]runtime.TypeID{
			1: runtime.ComplexRef(1),
			2: runtime.ComplexRef(1),
		},
	}
	if err := CheckElementDeclarationsConsistent(rt, rt.models[0]); err != nil {
		t.Fatalf("CheckElementDeclarationsConsistent() error = %v", err)
	}
}

func TestValidateContentRestrictionRejectsElementNillableLoosening(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	one := runtime.Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			runtime.ElementParticle(1, one),
			runtime.ElementParticle(2, one),
		),
		elementNames: map[runtime.ElementID]runtime.QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[runtime.ElementID]runtime.ParticleRestrictionElement{
			1: {Type: runtime.SimpleRef(1)},
			2: {Type: runtime.SimpleRef(1), Nillable: true},
		},
	}
	err := ValidateContentRestriction(rt, 0, 1)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
}

func TestValidateContentRestrictionAllowsSubstitutionMemberName(t *testing.T) {
	t.Parallel()

	headName := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	memberName := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 2}
	one := runtime.Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			runtime.ElementParticle(1, one),
			runtime.ElementParticle(2, one),
		),
		elementNames: map[runtime.ElementID]runtime.QName{
			1: headName,
			2: memberName,
		},
		elementRestrictions: map[runtime.ElementID]runtime.ParticleRestrictionElement{
			1: {Type: runtime.SimpleRef(1)},
			2: {Type: runtime.SimpleRef(1)},
		},
		substitutions: map[runtime.ElementID]map[runtime.QName]runtime.ElementID{
			1: {memberName: 2},
		},
	}
	if err := ValidateContentRestriction(rt, 0, 1); err != nil {
		t.Fatalf("ValidateContentRestriction() error = %v", err)
	}
}

func TestValidateContentRestrictionRejectsFixedValueMismatch(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	one := runtime.Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			runtime.ElementParticle(1, one),
			runtime.ElementParticle(2, one),
		),
		elementNames: map[runtime.ElementID]runtime.QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[runtime.ElementID]runtime.ParticleRestrictionElement{
			1: {Type: runtime.SimpleRef(1), Fixed: fixedValueConstraint("base", "base")},
			2: {Type: runtime.SimpleRef(1), Fixed: fixedValueConstraint("derived", "derived")},
		},
	}
	err := ValidateContentRestriction(rt, 0, 1)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
}

func TestValidateContentRestrictionAllowsFixedCanonicalMatch(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	one := runtime.Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			runtime.ElementParticle(1, one),
			runtime.ElementParticle(2, one),
		),
		elementNames: map[runtime.ElementID]runtime.QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[runtime.ElementID]runtime.ParticleRestrictionElement{
			1: {Type: runtime.SimpleRef(1), Fixed: fixedValueConstraint("1 2 3", "1 2 3")},
			2: {Type: runtime.SimpleRef(1), Fixed: fixedValueConstraint("1   2   3", "1 2 3")},
		},
	}
	if err := ValidateContentRestriction(rt, 0, 1); err != nil {
		t.Fatalf("ValidateContentRestriction() error = %v", err)
	}
}

func TestValidateContentRestrictionAllowsFixedValueIdentityMatch(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	one := runtime.Occurrence{Min: 1, Max: 1}
	identity := runtime.SimpleIdentityKey(runtime.PrimitiveDecimal, "5")
	rt := compiledModelRuntimeStub{
		models: restrictionModels(
			runtime.ElementParticle(1, one),
			runtime.ElementParticle(2, one),
		),
		elementNames: map[runtime.ElementID]runtime.QName{
			1: name,
			2: name,
		},
		elementRestrictions: map[runtime.ElementID]runtime.ParticleRestrictionElement{
			1: {Type: runtime.SimpleRef(1), Fixed: fixedValueConstraintWithIdentity("5.0", "5.0", 1, identity)},
			2: {Type: runtime.SimpleRef(2), Fixed: fixedValueConstraintWithIdentity("5", "5", 2, identity)},
		},
		simpleDerivations: map[runtime.SimpleTypeID]runtime.SimpleTypeDerivation{
			1: {Base: runtime.NoSimpleType, Variety: runtime.SimpleVarietyAtomic},
			2: {Base: 1, Variety: runtime.SimpleVarietyAtomic},
		},
	}
	if err := ValidateContentRestriction(rt, 0, 1); err != nil {
		t.Fatalf("ValidateContentRestriction() error = %v", err)
	}
}

func TestValidateContentRestrictionRejectsWildcardOutsideBase(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: 1, Local: 1}
	one := runtime.Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[runtime.ContentModelID]runtime.ContentModel{
			0: {
				Kind:      runtime.ModelSequence,
				Particles: []runtime.Particle{runtime.WildcardParticle(1, one)},
				Occurs:    one,
			},
			1: {
				Kind:      runtime.ModelSequence,
				Particles: []runtime.Particle{runtime.ElementParticle(1, one)},
				Occurs:    one,
			},
		},
		elementNames: map[runtime.ElementID]runtime.QName{
			1: name,
		},
		elementRestrictions: map[runtime.ElementID]runtime.ParticleRestrictionElement{
			1: {Type: runtime.SimpleRef(1)},
		},
		wildcards: map[runtime.WildcardID]runtime.Wildcard{
			1: {Mode: runtime.WildcardLocal},
		},
	}
	err := ValidateContentRestriction(rt, 0, 1)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
}

func TestValidateContentRestrictionMissingModelIsInternalInvariant(t *testing.T) {
	t.Parallel()

	one := runtime.Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{
		models: map[runtime.ContentModelID]runtime.ContentModel{
			0: {
				Kind:      runtime.ModelSequence,
				Particles: []runtime.Particle{runtime.ElementParticle(1, one)},
				Occurs:    one,
			},
		},
	}
	err := ValidateContentRestriction(rt, 0, 1)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestDeterministicRowCapsTransitionGroupBeforeStateLookup(t *testing.T) {
	b := &dfaBuilder{
		limit: 1,
		rows: []dfaSourceRow{{
			Edges: []dfaSourceEdge{
				{Particle: runtime.Particle{Kind: runtime.ParticleElement, Element: 1}, To: 0},
				{Particle: runtime.Particle{Kind: runtime.ParticleElement, Element: 1}, To: 1},
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
	b := &dfaBuilder{
		limit: 1,
		rows: []dfaSourceRow{{
			Edges: []dfaSourceEdge{
				{Particle: runtime.Particle{Kind: runtime.ParticleElement, Element: 1}, To: 0},
				{Particle: runtime.Particle{Kind: runtime.ParticleElement, Element: 1}, To: 0},
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

func expectDiagnostic(t *testing.T, err error, category xsderrors.Category, code xsderrors.Code) {
	t.Helper()
	var diag *xsderrors.Error
	if !errors.As(err, &diag) {
		t.Fatalf("error = %v, want xsderrors.Error", err)
	}
	if diag.Category != category || diag.Code != code {
		t.Fatalf("diagnostic = (%s, %s), want (%s, %s)", diag.Category, diag.Code, category, code)
	}
}

type compiledModelRuntimeStub struct {
	models              map[runtime.ContentModelID]runtime.ContentModel
	elementNames        map[runtime.ElementID]runtime.QName
	elementTypes        map[runtime.ElementID]runtime.TypeID
	elementRestrictions map[runtime.ElementID]runtime.ParticleRestrictionElement
	wildcards           map[runtime.WildcardID]runtime.Wildcard
	substitutions       map[runtime.ElementID]map[runtime.QName]runtime.ElementID
	simpleDerivations   map[runtime.SimpleTypeID]runtime.SimpleTypeDerivation
	complexDerivations  map[runtime.ComplexTypeID]runtime.ComplexTypeDerivation
	anyType             runtime.ComplexTypeID
}

func restrictionModels(base, derived runtime.Particle) map[runtime.ContentModelID]runtime.ContentModel {
	one := runtime.Occurrence{Min: 1, Max: 1}
	return map[runtime.ContentModelID]runtime.ContentModel{
		0: {
			Kind:      runtime.ModelSequence,
			Particles: []runtime.Particle{base},
			Occurs:    one,
		},
		1: {
			Kind:      runtime.ModelSequence,
			Particles: []runtime.Particle{derived},
			Occurs:    one,
		},
	}
}

func fixedValueConstraint(lexical, canonical string) runtime.ValueConstraintIdentity {
	return fixedValueConstraintWithIdentity(lexical, canonical, 1, "")
}

func fixedValueConstraintWithIdentity(
	lexical, canonical string,
	typ runtime.SimpleTypeID,
	identity string,
) runtime.ValueConstraintIdentity {
	return runtime.ValueConstraintIdentity{
		Lexical:   lexical,
		Canonical: canonical,
		Value:     runtime.SimpleValue{Canonical: canonical, Identity: identity, Type: typ},
		Present:   true,
	}
}

func compiledModelRuntimeFixture(t *testing.T, kind runtime.ModelKind) (runtime.NameTable, compiledModelRuntimeStub) {
	t.Helper()

	var required []runtime.ExpandedName
	elementNames := make(map[runtime.ElementID]runtime.QName)
	particles := make([]runtime.Particle, runtime.CompiledDFARowIndexMinEdges)
	for i := range particles {
		local := string(rune('a' + i))
		required = append(required, runtime.ExpandedName{Local: local})
	}
	names, err := runtime.NewNameTable(64, []string{runtime.EmptyNamespaceURI}, required)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	one := runtime.Occurrence{Min: 1, Max: 1}
	for i := range particles {
		local := string(rune('a' + i))
		name, ok := names.LookupQName("", local)
		if !ok {
			t.Fatalf("QName %q missing", local)
		}
		id := runtime.ElementID(i + 1)
		elementNames[id] = name
		particles[i] = runtime.ElementParticle(id, one)
	}
	rt := compiledModelRuntimeStub{
		models: map[runtime.ContentModelID]runtime.ContentModel{
			0: {Kind: kind, Occurs: one, Particles: particles},
		},
		elementNames: elementNames,
		elementTypes: make(map[runtime.ElementID]runtime.TypeID, len(elementNames)),
	}
	for id := range elementNames {
		rt.elementTypes[id] = runtime.SimpleRef(runtime.SimpleTypeID(id))
	}
	return names, rt
}

func (s compiledModelRuntimeStub) ContentModel(id runtime.ContentModelID) (runtime.ContentModel, bool) {
	model, ok := s.models[id]
	return model, ok
}

func (s compiledModelRuntimeStub) ElementName(id runtime.ElementID) (runtime.QName, bool) {
	name, ok := s.elementNames[id]
	return name, ok
}

func (s compiledModelRuntimeStub) ElementType(id runtime.ElementID) (runtime.TypeID, bool) {
	if decl, ok := s.elementRestrictions[id]; ok {
		return decl.Type, true
	}
	typ, ok := s.elementTypes[id]
	return typ, ok
}

func (s compiledModelRuntimeStub) ElementRestriction(id runtime.ElementID) (runtime.ParticleRestrictionElement, bool) {
	if decl, ok := s.elementRestrictions[id]; ok {
		return decl, true
	}
	typ, ok := s.elementTypes[id]
	if !ok {
		return runtime.ParticleRestrictionElement{}, false
	}
	return runtime.ParticleRestrictionElement{Type: typ}, true
}

func (s compiledModelRuntimeStub) Wildcard(id runtime.WildcardID) (runtime.Wildcard, bool) {
	wildcard, ok := s.wildcards[id]
	return wildcard, ok
}

func (s compiledModelRuntimeStub) ForEachSubstitutionMember(id runtime.ElementID, fn func(runtime.ElementID) bool) {
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

func (s compiledModelRuntimeStub) SubstitutionMemberByName(id runtime.ElementID, name runtime.QName) (runtime.ElementID, bool) {
	members := s.substitutions[id]
	if members == nil {
		return runtime.NoElement, false
	}
	member, ok := members[name]
	return member, ok
}

func (s compiledModelRuntimeStub) SubstitutionMembersByName(id runtime.ElementID) map[runtime.QName]runtime.ElementID {
	return s.substitutions[id]
}

func (s compiledModelRuntimeStub) AnyTypeID() runtime.ComplexTypeID {
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

func (s compiledModelRuntimeStub) SimpleTypeDerivation(id runtime.SimpleTypeID) (runtime.SimpleTypeDerivation, bool) {
	derivation, ok := s.simpleDerivations[id]
	return derivation, ok
}

func (s compiledModelRuntimeStub) ComplexTypeDerivation(id runtime.ComplexTypeID) (runtime.ComplexTypeDerivation, bool) {
	derivation, ok := s.complexDerivations[id]
	return derivation, ok
}
