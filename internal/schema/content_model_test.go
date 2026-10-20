package schema

import (
	"errors"
	"slices"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func TestContentModelByID(t *testing.T) {
	t.Parallel()

	models := []ContentModel{
		{Kind: ModelEmpty},
		{
			Kind:      ModelSequence,
			Occurs:    Occurrence{Min: 1, Max: 1},
			Particles: []Particle{ElementParticle(1, Occurrence{Min: 1, Max: 1})},
		},
	}
	got, ok := ContentModelByID(models, 1)
	if !ok ||
		got.Kind != ModelSequence ||
		!slices.Equal(got.Particles, models[1].Particles) {
		t.Fatalf("ContentModelByID(valid) = %+v, %v; want cloned model 1, true", got, ok)
	}
	got.Particles[0].Element = 9
	if models[1].Particles[0].Element != 1 {
		t.Fatalf("ContentModelByID aliased particle slice: %+v", models[1])
	}
	for _, id := range []ContentModelID{NoContentModel, 2} {
		got, ok := ContentModelByID(models, id)
		if ok || got.Kind != 0 || len(got.Particles) != 0 {
			t.Fatalf("ContentModelByID(%d) = %+v, %v; want zero, false", id, got, ok)
		}
	}
}

func TestValidateContentModelShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		in      ContentModel
	}{
		{
			name: "empty",
			in:   ContentModel{Kind: ModelEmpty},
		},
		{
			name: "any",
			in:   ContentModel{Kind: ModelAny, Mixed: true},
		},
		{
			name: "sequence",
			in: ContentModel{
				Kind:      ModelSequence,
				Occurs:    Occurrence{Min: 1, Max: 1},
				Particles: []Particle{ElementParticle(1, Occurrence{Min: 1, Max: 1})},
			},
		},
		{
			name: "choice",
			in: ContentModel{
				Kind:      ModelChoice,
				Occurs:    Occurrence{Min: 0, Unbounded: true},
				Particles: []Particle{ElementParticle(1, Occurrence{Min: 1, Max: 1})},
			},
		},
		{
			name: "all",
			in: ContentModel{
				Kind:      ModelAll,
				Occurs:    Occurrence{Min: 0, Max: 1},
				Particles: []Particle{ElementParticle(1, Occurrence{Min: 0, Max: 1})},
			},
		},
		{
			name: "empty stores inactive fields",
			in: ContentModel{
				Kind:      ModelEmpty,
				Particles: []Particle{ElementParticle(1, Occurrence{Min: 1, Max: 1})},
			},
			wantErr: "empty content model stores inactive fields",
		},
		{
			name:    "any requires mixed",
			in:      ContentModel{Kind: ModelAny},
			wantErr: "any content model has invalid shape",
		},
		{
			name:    "sequence invalid occurrence",
			in:      ContentModel{Kind: ModelSequence, Occurs: Occurrence{Min: 2, Max: 1}},
			wantErr: "content model occurrence is invalid",
		},
		{
			name:    "all cannot be unbounded",
			in:      ContentModel{Kind: ModelAll, Occurs: Occurrence{Min: 0, Unbounded: true}},
			wantErr: "all content model occurrence is invalid",
		},
		{
			name:    "invalid kind",
			in:      ContentModel{Kind: ModelKind(99)},
			wantErr: "content model has invalid kind",
		},
		{
			name: "particle shape",
			in: ContentModel{
				Kind:      ModelSequence,
				Occurs:    Occurrence{Min: 1, Max: 1},
				Particles: []Particle{{Kind: ParticleElement, Element: 1, Model: 2, Occurs: Occurrence{Min: 1, Max: 1}}},
			},
			wantErr: "particle stores content model ID for non-model kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateContentModelShape(tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateContentModelShape() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateContentModelShape() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContentModelRuntimeRejectsInvalidParticleReferences(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	limits := ContentModelRefLimits{
		ElementCount:      1,
		ContentModelCount: 1,
		WildcardCount:     1,
	}
	tests := []struct {
		name    string
		wantErr string
		model   ContentModel
	}{
		{
			name: "valid element",
			model: ContentModel{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{ElementParticle(0, one)},
			},
		},
		{
			name: "invalid element",
			model: ContentModel{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{ElementParticle(1, one)},
			},
			wantErr: "particle references invalid element",
		},
		{
			name: "invalid content model",
			model: ContentModel{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{ModelParticle(1, one)},
			},
			wantErr: "particle references invalid content model",
		},
		{
			name: "invalid wildcard",
			model: ContentModel{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{WildcardParticle(1, one)},
			},
			wantErr: "particle references invalid wildcard",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateContentModelRuntime(tt.model, limits)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateContentModelRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateContentModelRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContentModelGraphRejectsCycles(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	model := func(children ...ContentModelID) ContentModel {
		particles := make([]Particle, len(children))
		for i, child := range children {
			particles[i] = ModelParticle(child, one)
		}
		return ContentModel{Kind: ModelSequence, Occurs: one, Particles: particles}
	}
	tests := []struct {
		name    string
		wantErr string
		models  []ContentModel
	}{
		{name: "acyclic", models: []ContentModel{model(1), {Kind: ModelEmpty}}},
		{name: "self cycle", models: []ContentModel{model(0)}, wantErr: "content model graph contains cycle"},
		{name: "multi-node cycle", models: []ContentModel{model(1), model(0)}, wantErr: "content model graph contains cycle"},
		{name: "invalid reference", models: []ContentModel{model(1)}, wantErr: "content model graph references invalid model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateContentModelGraph(tt.models)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateContentModelGraph() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("validateContentModelGraph() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContentModelGraphHandlesDeepChain(t *testing.T) {
	t.Parallel()

	const count = 10_000
	one := Occurrence{Min: 1, Max: 1}
	models := make([]ContentModel, count)
	for i := range count - 1 {
		models[i] = ContentModel{
			Kind:      ModelSequence,
			Occurs:    one,
			Particles: []Particle{ModelParticle(ContentModelID(i+1), one)},
		}
	}
	models[count-1] = ContentModel{Kind: ModelEmpty}
	if err := validateContentModelGraph(models); err != nil {
		t.Fatalf("validateContentModelGraph() error = %v", err)
	}
}

func TestValidateContentModelGraphHandlesWideFlatTable(t *testing.T) {
	t.Parallel()

	models := make([]ContentModel, 10_000)
	for i := range models {
		models[i] = ContentModel{Kind: ModelEmpty}
	}
	if err := validateContentModelGraph(models); err != nil {
		t.Fatalf("validateContentModelGraph() error = %v", err)
	}
}

func TestComplexContentExtendsBase(t *testing.T) {
	t.Parallel()

	const (
		emptyID      ContentModelID = 0
		baseSeqID    ContentModelID = 1
		derivedSeqID ContentModelID = 2
		baseChoiceID ContentModelID = 3
		wrappedID    ContentModelID = 4
		badKindID    ContentModelID = 5
		badOccursID  ContentModelID = 6
		badPrefixID  ContentModelID = 7
		elemA        ElementID      = 0
		elemB        ElementID      = 1
	)
	one := Occurrence{Min: 1, Max: 1}
	repeat := Occurrence{Min: 0, Unbounded: true}
	baseParticle := ElementParticle(elemA, one)
	nextParticle := ElementParticle(elemB, one)
	rt := particleFixture{
		models: []ContentModel{
			{Kind: ModelEmpty},
			{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{baseParticle},
			},
			{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{baseParticle, nextParticle},
			},
			{
				Kind:      ModelChoice,
				Occurs:    one,
				Particles: []Particle{baseParticle, nextParticle},
			},
			{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{ModelParticle(baseChoiceID, one), nextParticle},
			},
			{
				Kind:      ModelChoice,
				Occurs:    one,
				Particles: []Particle{baseParticle, nextParticle},
			},
			{
				Kind:      ModelSequence,
				Occurs:    repeat,
				Particles: []Particle{baseParticle, nextParticle},
			},
			{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{nextParticle, baseParticle},
			},
		},
	}
	tests := []struct {
		name    string
		base    ContentModelID
		derived ContentModelID
		want    bool
	}{
		{name: "same model", base: baseSeqID, derived: baseSeqID, want: true},
		{name: "empty base", base: emptyID, derived: badKindID, want: true},
		{name: "sequence prefix", base: baseSeqID, derived: derivedSeqID, want: true},
		{name: "wrapped non-sequence base", base: baseChoiceID, derived: wrappedID, want: true},
		{name: "invalid base", base: 99, derived: derivedSeqID},
		{name: "invalid derived", base: baseSeqID, derived: 99},
		{name: "derived must be sequence", base: baseSeqID, derived: badKindID},
		{name: "derived must occur exactly once", base: baseSeqID, derived: badOccursID},
		{name: "derived sequence must preserve prefix", base: baseSeqID, derived: badPrefixID},
		{name: "wrapped base must be first particle", base: baseChoiceID, derived: derivedSeqID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ComplexContentExtendsBase(rt.build(), tt.base, tt.derived); got != tt.want {
				t.Fatalf("ComplexContentExtendsBase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContentRestrictionRejectsRepeatedElementAgainstUnitChoiceBranch(t *testing.T) {
	t.Parallel()
	const (
		choiceID  ContentModelID = 0
		baseID    ContentModelID = 1
		derivedID ContentModelID = 2
	)
	one := Occurrence{Min: 1, Max: 1}
	repeat := Occurrence{Min: 0, Unbounded: true}
	models := []ContentModel{
		{Kind: ModelChoice, Occurs: one, Particles: []Particle{ElementParticle(1, one), ElementParticle(2, one)}},
		{Kind: ModelSequence, Occurs: one, Particles: []Particle{ModelParticle(choiceID, repeat)}},
		{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(1, repeat)}},
	}
	rt := contentRestrictionRuntimeWith(models)
	if err := ValidateContentRestriction(rt.build(), baseID, derivedID, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); !IsContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want repeated element/unit branch mismatch", err)
	}
}

func TestContentRestrictionUsesFixedValueIdentity(t *testing.T) {
	t.Parallel()

	const (
		baseType    SimpleTypeID = 1
		derivedType SimpleTypeID = 2
	)
	one := Occurrence{Min: 1, Max: 1}
	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		models: []ContentModel{
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(0, one)}},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(1, one)}},
		},
		elements: []QName{name, name},
		elementRestrictions: []ParticleRestrictionElement{
			{Type: SimpleRef(baseType), Fixed: valueConstraintIdentity(t, "decimal", "5.0"), Scope: DeclarationScopeNonGlobal},
			{Type: SimpleRef(derivedType), Fixed: valueConstraintIdentity(t, "integer", "5"), Scope: DeclarationScopeNonGlobal},
		},
		simpleDerivations: []SimpleTypeDerivation{
			{},
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
			{Base: baseType, Variety: valuepkg.Atomic},
		},
	}
	if err := ValidateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); err != nil {
		t.Fatalf("ValidateContentRestriction() error = %v", err)
	}

	rt.elementRestrictions[1].Fixed.Value = builtinValueForTest(t, "string", "5")
	if err := ValidateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); err == nil {
		t.Fatal("ValidateContentRestriction() accepted different fixed value identities")
	}
}

func TestContentRestrictionRequiresLocalIdentitySubset(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		models: []ContentModel{
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(0, one)}},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(1, one)}},
		},
		elements: []QName{name, name},
		elementRestrictions: []ParticleRestrictionElement{
			{Type: SimpleRef(0), Scope: DeclarationScopeNonGlobal, Identities: borrowedIdentityConstraintIDs([]IdentityConstraintID{1, 2})},
			{Type: SimpleRef(0), Scope: DeclarationScopeNonGlobal, Identities: borrowedIdentityConstraintIDs([]IdentityConstraintID{2})},
		},
		simpleDerivations: []SimpleTypeDerivation{{Base: NoSimpleType, Variety: valuepkg.Atomic}},
	}
	if err := ValidateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); err != nil {
		t.Fatalf("ValidateContentRestriction(subset) error = %v", err)
	}
	rt.elementRestrictions[1].Identities = borrowedIdentityConstraintIDs([]IdentityConstraintID{3})
	if err := ValidateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); err == nil {
		t.Fatal("ValidateContentRestriction() accepted a local identity constraint outside the base set")
	}
}

func TestContentRestrictionHonorsWorkBudget(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		models: []ContentModel{
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(0, one)}},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(1, one)}},
		},
		elements: []QName{name, name},
		elementRestrictions: []ParticleRestrictionElement{
			{Type: SimpleRef(0), Scope: DeclarationScopeNonGlobal},
			{Type: SimpleRef(0), Scope: DeclarationScopeNonGlobal},
		},
		simpleDerivations: []SimpleTypeDerivation{{Base: NoSimpleType, Variety: valuepkg.Atomic}},
	}
	budgetExceeded := errors.New("content work exceeded")
	remaining := 2
	work := func(steps int) error {
		remaining -= steps
		if remaining < 0 {
			return budgetExceeded
		}
		return nil
	}
	analysis, err := NewContentModelAnalysis(rt.build(), work)
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateContentRestriction(rt.build(), 0, 1, work, analysis)
	if !errors.Is(err, budgetExceeded) {
		t.Fatalf("ValidateContentRestriction() error = %v, want budget error", err)
	}
}

func TestContentRestrictionChargesTypeDerivation(t *testing.T) {
	t.Parallel()

	const depth = 32
	derivations := make([]SimpleTypeDerivation, depth)
	derivations[0] = SimpleTypeDerivation{Base: NoSimpleType, Variety: valuepkg.Atomic}
	for id := 1; id < depth; id++ {
		derivations[id] = SimpleTypeDerivation{Base: SimpleTypeID(id - 1), Variety: valuepkg.Atomic}
	}
	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		elements: []QName{name, name},
		elementRestrictions: []ParticleRestrictionElement{
			{Type: SimpleRef(0), Scope: DeclarationScopeNonGlobal},
			{Type: SimpleRef(depth - 1), Scope: DeclarationScopeNonGlobal},
		},
		simpleDerivations: derivations,
	}
	budgetExceeded := errors.New("content work exceeded")
	remaining := 8
	work := func(steps int) error {
		if steps > remaining {
			return budgetExceeded
		}
		remaining -= steps
		return nil
	}
	analysis, err := NewContentModelAnalysis(rt.build(), work)
	if err != nil {
		t.Fatal(err)
	}
	validator := newContentRestrictionValidator(rt.build(), work, analysis)
	one := Occurrence{Min: 1, Max: 1}
	err = validator.validateParticleRestrictsElement(ElementParticle(0, one), ElementParticle(1, one))
	if !errors.Is(err, budgetExceeded) {
		t.Fatalf("validateParticleRestrictsElement() error = %v, want budget error", err)
	}
}

func TestContentRestrictionRejectsInvalidElementScope(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		models: []ContentModel{
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(0, one)}},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(1, one)}},
		},
		elements: []QName{name, name},
		elementRestrictions: []ParticleRestrictionElement{
			{Type: SimpleRef(0), Scope: DeclarationScopeNonGlobal},
			{Type: SimpleRef(0), Scope: DeclarationScopeInvalid},
		},
		simpleDerivations: []SimpleTypeDerivation{{Base: NoSimpleType, Variety: valuepkg.Atomic}},
	}
	err := ValidateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build()))
	if !IsContentRestrictionInvariant(err) || !strings.Contains(err.Error(), "invalid scope") {
		t.Fatalf("ValidateContentRestriction() error = %v, want invalid-scope invariant", err)
	}
}

func TestContentRestrictionPropagatesMissingNestedModel(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	models := []ContentModel{
		{Kind: ModelSequence, Occurs: one, Particles: []Particle{ModelParticle(99, one)}},
		{Kind: ModelSequence, Occurs: one},
	}
	rt := contentRestrictionRuntimeWith(models)
	err := ValidateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build()))
	if !IsContentRestrictionInvariant(err) || !strings.Contains(err.Error(), "missing content model") {
		t.Fatalf("ValidateContentRestriction() error = %v, want missing-model invariant", err)
	}
}

func TestContentRestrictionRejectsRepeatedUnitChoiceBranchWithTail(t *testing.T) {
	t.Parallel()
	const (
		choiceID  ContentModelID = 0
		baseID    ContentModelID = 1
		derivedID ContentModelID = 2
		elemA     ElementID      = 1
		elemB     ElementID      = 2
	)
	one := Occurrence{Min: 1, Max: 1}
	repeat := Occurrence{Min: 0, Unbounded: true}
	models := []ContentModel{
		{Kind: ModelChoice, Occurs: one, Particles: []Particle{ElementParticle(elemA, one), ElementParticle(elemB, one)}},
		{Kind: ModelSequence, Occurs: one, Particles: []Particle{ModelParticle(choiceID, repeat), ElementParticle(elemB, one)}},
		{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(elemA, repeat), ElementParticle(elemB, repeat)}},
	}
	rt := contentRestrictionRuntimeWith(models)
	if err := ValidateContentRestriction(rt.build(), baseID, derivedID, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); !IsContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want repeated element/unit branch mismatch", err)
	}
}

func contentRestrictionRuntimeWith(models []ContentModel) contentRestrictionFixture {
	nameA := QName{Namespace: EmptyNamespaceID, Local: 1}
	nameB := QName{Namespace: EmptyNamespaceID, Local: 2}
	return contentRestrictionFixture{
		models:   models,
		elements: []QName{{}, nameA, nameB},
		elementRestrictions: []ParticleRestrictionElement{
			{},
			{Type: ComplexRef(0), Scope: DeclarationScopeNonGlobal},
			{Type: ComplexRef(0), Scope: DeclarationScopeNonGlobal},
		},
		anyType: 0,
	}
}

type contentRestrictionFixture struct {
	models              []ContentModel
	elements            []QName
	elementRestrictions []ParticleRestrictionElement
	complexDerivations  []ComplexTypeDerivation
	simpleDerivations   []SimpleTypeDerivation
	anyType             ComplexTypeID
}

func TestValidateParticleShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		in      Particle
	}{
		{
			name: "element constructor",
			in:   ElementParticle(1, Occurrence{Min: 1, Max: 1}),
		},
		{
			name: "model constructor",
			in:   ModelParticle(2, Occurrence{Min: 0, Max: 1}),
		},
		{
			name: "wildcard constructor",
			in:   WildcardParticle(3, Occurrence{Min: 0, Unbounded: true}),
		},
		{
			name:    "invalid occurrence",
			in:      ElementParticle(1, Occurrence{Min: 1, Max: 1, Unbounded: true}),
			wantErr: "particle occurrence is invalid",
		},
		{
			name:    "invalid kind",
			in:      Particle{Kind: ParticleKind(99), Occurs: Occurrence{Min: 1, Max: 1}},
			wantErr: "particle has invalid kind",
		},
		{
			name:    "element stores model",
			in:      Particle{Kind: ParticleElement, Element: 1, Model: 2, Wildcard: NoWildcard, Occurs: Occurrence{Min: 1, Max: 1}},
			wantErr: "particle stores content model ID for non-model kind",
		},
		{
			name:    "model stores element",
			in:      Particle{Kind: ParticleModel, Element: 1, Model: 2, Wildcard: NoWildcard, Occurs: Occurrence{Min: 1, Max: 1}},
			wantErr: "particle stores element ID for non-element kind",
		},
		{
			name:    "wildcard stores model",
			in:      Particle{Kind: ParticleWildcard, Element: NoElement, Model: 2, Wildcard: 3, Occurs: Occurrence{Min: 1, Max: 1}},
			wantErr: "particle stores content model ID for non-model kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateParticleShape(tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateParticleShape() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateParticleShape() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestOccurrenceIsExactlyOne(t *testing.T) {
	t.Parallel()

	if !((Occurrence{Min: 1, Max: 1}).IsExactlyOne()) {
		t.Fatal("1..1 occurrence should be exactly one")
	}
	for _, o := range []Occurrence{
		{},
		{Min: 0, Max: 1},
		{Min: 1, Unbounded: true},
	} {
		if o.IsExactlyOne() {
			t.Fatalf("%+v should not be exactly one", o)
		}
	}
}

func (f contentRestrictionFixture) build() *schemaBuild {
	b := (&derivationFixture{simple: f.simpleDerivations, complex: f.complexDerivations, anyType: f.anyType}).build()
	b.Models = cloneContentModels(f.models)
	b.Elements = fixtureElements(f.elements, f.elementRestrictions)
	return b
}
