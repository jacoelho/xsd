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

	models := []contentModelNode{
		{Kind: modelEmpty},
		{
			Kind:      modelSequence,
			Occurs:    occurrence{Min: 1, Max: 1},
			Particles: []contentParticle{elementParticle(1, occurrence{Min: 1, Max: 1})},
		},
	}
	got, ok := contentModelByID(models, 1)
	if !ok ||
		got.Kind != modelSequence ||
		!slices.Equal(got.Particles, models[1].Particles) {
		t.Fatalf("ContentModelByID(valid) = %+v, %v; want stored model 1, true", got, ok)
	}
	if &got.Particles[0] != &models[1].Particles[0] {
		t.Fatalf("ContentModelByID copied the stored particle slice: %+v", models[1])
	}
	for _, id := range []contentModelID{noContentModel, 2} {
		got, ok := contentModelByID(models, id)
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
		in      contentModelNode
	}{
		{
			name: "empty",
			in:   contentModelNode{Kind: modelEmpty},
		},
		{
			name: "any",
			in:   contentModelNode{Kind: modelAny, Mixed: true},
		},
		{
			name: "sequence",
			in: contentModelNode{
				Kind:      modelSequence,
				Occurs:    occurrence{Min: 1, Max: 1},
				Particles: []contentParticle{elementParticle(1, occurrence{Min: 1, Max: 1})},
			},
		},
		{
			name: "choice",
			in: contentModelNode{
				Kind:      modelChoice,
				Occurs:    occurrence{Min: 0, Unbounded: true},
				Particles: []contentParticle{elementParticle(1, occurrence{Min: 1, Max: 1})},
			},
		},
		{
			name: "all",
			in: contentModelNode{
				Kind:      modelAll,
				Occurs:    occurrence{Min: 0, Max: 1},
				Particles: []contentParticle{elementParticle(1, occurrence{Min: 0, Max: 1})},
			},
		},
		{
			name: "empty stores inactive fields",
			in: contentModelNode{
				Kind:      modelEmpty,
				Particles: []contentParticle{elementParticle(1, occurrence{Min: 1, Max: 1})},
			},
			wantErr: "empty content model stores inactive fields",
		},
		{
			name:    "any requires mixed",
			in:      contentModelNode{Kind: modelAny},
			wantErr: "any content model has invalid shape",
		},
		{
			name:    "sequence invalid occurrence",
			in:      contentModelNode{Kind: modelSequence, Occurs: occurrence{Min: 2, Max: 1}},
			wantErr: "content model occurrence is invalid",
		},
		{
			name:    "all cannot be unbounded",
			in:      contentModelNode{Kind: modelAll, Occurs: occurrence{Min: 0, Unbounded: true}},
			wantErr: "all content model occurrence is invalid",
		},
		{
			name:    "invalid kind",
			in:      contentModelNode{Kind: modelKind(99)},
			wantErr: "content model has invalid kind",
		},
		{
			name: "particle shape",
			in: contentModelNode{
				Kind:      modelSequence,
				Occurs:    occurrence{Min: 1, Max: 1},
				Particles: []contentParticle{{Kind: particleElement, Element: 1, Model: 2, Occurs: occurrence{Min: 1, Max: 1}}},
			},
			wantErr: "particle stores content model ID for non-model kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateContentModelShape(tt.in)
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

	one := occurrence{Min: 1, Max: 1}
	limits := contentModelRefLimits{
		ElementCount:      1,
		ContentModelCount: 1,
		WildcardCount:     1,
	}
	tests := []struct {
		name    string
		wantErr string
		model   contentModelNode
	}{
		{
			name: "valid element",
			model: contentModelNode{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{elementParticle(0, one)},
			},
		},
		{
			name: "invalid element",
			model: contentModelNode{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{elementParticle(1, one)},
			},
			wantErr: "particle references invalid element",
		},
		{
			name: "invalid content model",
			model: contentModelNode{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{modelParticle(1, one)},
			},
			wantErr: "particle references invalid content model",
		},
		{
			name: "invalid wildcard",
			model: contentModelNode{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{wildcardParticle(1, one)},
			},
			wantErr: "particle references invalid wildcard",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateContentModelRuntime(tt.model, limits)
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

	one := occurrence{Min: 1, Max: 1}
	model := func(children ...contentModelID) contentModelNode {
		particles := make([]contentParticle, len(children))
		for i, child := range children {
			particles[i] = modelParticle(child, one)
		}
		return contentModelNode{Kind: modelSequence, Occurs: one, Particles: particles}
	}
	tests := []struct {
		name    string
		wantErr string
		models  []contentModelNode
	}{
		{name: "acyclic", models: []contentModelNode{model(1), {Kind: modelEmpty}}},
		{name: "self cycle", models: []contentModelNode{model(0)}, wantErr: "content model graph contains cycle"},
		{name: "multi-node cycle", models: []contentModelNode{model(1), model(0)}, wantErr: "content model graph contains cycle"},
		{name: "invalid reference", models: []contentModelNode{model(1)}, wantErr: "content model graph references invalid model"},
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
	one := occurrence{Min: 1, Max: 1}
	models := make([]contentModelNode, count)
	for i := range count - 1 {
		models[i] = contentModelNode{
			Kind:      modelSequence,
			Occurs:    one,
			Particles: []contentParticle{modelParticle(contentModelID(i+1), one)},
		}
	}
	models[count-1] = contentModelNode{Kind: modelEmpty}
	if err := validateContentModelGraph(models); err != nil {
		t.Fatalf("validateContentModelGraph() error = %v", err)
	}
}

func TestValidateContentModelGraphHandlesWideFlatTable(t *testing.T) {
	t.Parallel()

	models := make([]contentModelNode, 10_000)
	for i := range models {
		models[i] = contentModelNode{Kind: modelEmpty}
	}
	if err := validateContentModelGraph(models); err != nil {
		t.Fatalf("validateContentModelGraph() error = %v", err)
	}
}

func TestComplexContentExtendsBase(t *testing.T) {
	t.Parallel()

	const (
		emptyID      contentModelID = 0
		baseSeqID    contentModelID = 1
		derivedSeqID contentModelID = 2
		baseChoiceID contentModelID = 3
		wrappedID    contentModelID = 4
		badKindID    contentModelID = 5
		badOccursID  contentModelID = 6
		badPrefixID  contentModelID = 7
		elemA        ElementID      = 0
		elemB        ElementID      = 1
	)
	one := occurrence{Min: 1, Max: 1}
	repeat := occurrence{Min: 0, Unbounded: true}
	baseParticle := elementParticle(elemA, one)
	nextParticle := elementParticle(elemB, one)
	rt := particleFixture{
		models: []contentModelNode{
			{Kind: modelEmpty},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{baseParticle},
			},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{baseParticle, nextParticle},
			},
			{
				Kind:      modelChoice,
				Occurs:    one,
				Particles: []contentParticle{baseParticle, nextParticle},
			},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{modelParticle(baseChoiceID, one), nextParticle},
			},
			{
				Kind:      modelChoice,
				Occurs:    one,
				Particles: []contentParticle{baseParticle, nextParticle},
			},
			{
				Kind:      modelSequence,
				Occurs:    repeat,
				Particles: []contentParticle{baseParticle, nextParticle},
			},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{nextParticle, baseParticle},
			},
		},
	}
	tests := []struct {
		name    string
		base    contentModelID
		derived contentModelID
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

			if got := complexContentExtendsBase(rt.build(), tt.base, tt.derived); got != tt.want {
				t.Fatalf("ComplexContentExtendsBase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContentRestrictionRejectsRepeatedElementAgainstUnitChoiceBranch(t *testing.T) {
	t.Parallel()
	const (
		choiceID  contentModelID = 0
		baseID    contentModelID = 1
		derivedID contentModelID = 2
	)
	one := occurrence{Min: 1, Max: 1}
	repeat := occurrence{Min: 0, Unbounded: true}
	models := []contentModelNode{
		{Kind: modelChoice, Occurs: one, Particles: []contentParticle{elementParticle(1, one), elementParticle(2, one)}},
		{Kind: modelSequence, Occurs: one, Particles: []contentParticle{modelParticle(choiceID, repeat)}},
		{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(1, repeat)}},
	}
	rt := contentRestrictionRuntimeWith(models)
	if err := validateContentRestriction(rt.build(), baseID, derivedID, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); !isContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want repeated element/unit branch mismatch", err)
	}
}

func TestContentRestrictionUsesFixedValueIdentity(t *testing.T) {
	t.Parallel()

	const (
		baseType    SimpleTypeID = 1
		derivedType SimpleTypeID = 2
	)
	one := occurrence{Min: 1, Max: 1}
	name := QName{Namespace: emptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		models: []contentModelNode{
			{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(0, one)}},
			{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(1, one)}},
		},
		elements: []QName{name, name},
		elementRestrictions: []particleRestrictionElement{
			{Type: SimpleRef(baseType), Fixed: valueConstraintIdentity(t, "decimal", "5.0"), Scope: declarationScopeNonGlobal},
			{Type: SimpleRef(derivedType), Fixed: valueConstraintIdentity(t, "integer", "5"), Scope: declarationScopeNonGlobal},
		},
		simpleDerivations: []simpleTypeDerivation{
			{},
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
			{Base: baseType, Variety: valuepkg.Atomic},
		},
	}
	if err := validateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); err != nil {
		t.Fatalf("ValidateContentRestriction() error = %v", err)
	}

	rt.elementRestrictions[1].Fixed.Value = builtinValueForTest(t, "string", "5")
	if err := validateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); err == nil {
		t.Fatal("ValidateContentRestriction() accepted different fixed value identities")
	}
}

func TestContentRestrictionRequiresLocalIdentitySubset(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	name := QName{Namespace: emptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		models: []contentModelNode{
			{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(0, one)}},
			{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(1, one)}},
		},
		elements: []QName{name, name},
		elementRestrictions: []particleRestrictionElement{
			{Type: SimpleRef(0), Scope: declarationScopeNonGlobal, Identities: borrowedIdentityConstraintIDs([]IdentityConstraintID{1, 2})},
			{Type: SimpleRef(0), Scope: declarationScopeNonGlobal, Identities: borrowedIdentityConstraintIDs([]IdentityConstraintID{2})},
		},
		simpleDerivations: []simpleTypeDerivation{{Base: NoSimpleType, Variety: valuepkg.Atomic}},
	}
	if err := validateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); err != nil {
		t.Fatalf("ValidateContentRestriction(subset) error = %v", err)
	}
	rt.elementRestrictions[1].Identities = borrowedIdentityConstraintIDs([]IdentityConstraintID{3})
	if err := validateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); err == nil {
		t.Fatal("ValidateContentRestriction() accepted a local identity constraint outside the base set")
	}
}

func TestContentRestrictionHonorsWorkBudget(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	name := QName{Namespace: emptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		models: []contentModelNode{
			{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(0, one)}},
			{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(1, one)}},
		},
		elements: []QName{name, name},
		elementRestrictions: []particleRestrictionElement{
			{Type: SimpleRef(0), Scope: declarationScopeNonGlobal},
			{Type: SimpleRef(0), Scope: declarationScopeNonGlobal},
		},
		simpleDerivations: []simpleTypeDerivation{{Base: NoSimpleType, Variety: valuepkg.Atomic}},
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
	analysis, err := newContentModelAnalysis(rt.build(), work)
	if err != nil {
		t.Fatal(err)
	}
	err = validateContentRestriction(rt.build(), 0, 1, work, analysis)
	if !errors.Is(err, budgetExceeded) {
		t.Fatalf("ValidateContentRestriction() error = %v, want budget error", err)
	}
}

func TestContentRestrictionChargesTypeDerivation(t *testing.T) {
	t.Parallel()

	const depth = 32
	derivations := make([]simpleTypeDerivation, depth)
	derivations[0] = simpleTypeDerivation{Base: NoSimpleType, Variety: valuepkg.Atomic}
	for id := 1; id < depth; id++ {
		derivations[id] = simpleTypeDerivation{Base: SimpleTypeID(id - 1), Variety: valuepkg.Atomic}
	}
	name := QName{Namespace: emptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		elements: []QName{name, name},
		elementRestrictions: []particleRestrictionElement{
			{Type: SimpleRef(0), Scope: declarationScopeNonGlobal},
			{Type: SimpleRef(depth - 1), Scope: declarationScopeNonGlobal},
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
	analysis, err := newContentModelAnalysis(rt.build(), work)
	if err != nil {
		t.Fatal(err)
	}
	validator := newContentRestrictionValidator(rt.build(), work, analysis)
	one := occurrence{Min: 1, Max: 1}
	err = validator.validateParticleRestrictsElement(elementParticle(0, one), elementParticle(1, one))
	if !errors.Is(err, budgetExceeded) {
		t.Fatalf("validateParticleRestrictsElement() error = %v, want budget error", err)
	}
}

func TestContentRestrictionRejectsInvalidElementScope(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	name := QName{Namespace: emptyNamespaceID, Local: 1}
	rt := contentRestrictionFixture{
		models: []contentModelNode{
			{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(0, one)}},
			{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(1, one)}},
		},
		elements: []QName{name, name},
		elementRestrictions: []particleRestrictionElement{
			{Type: SimpleRef(0), Scope: declarationScopeNonGlobal},
			{Type: SimpleRef(0), Scope: declarationScopeInvalid},
		},
		simpleDerivations: []simpleTypeDerivation{{Base: NoSimpleType, Variety: valuepkg.Atomic}},
	}
	err := validateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build()))
	if !isContentRestrictionInvariant(err) || !strings.Contains(err.Error(), "invalid scope") {
		t.Fatalf("ValidateContentRestriction() error = %v, want invalid-scope invariant", err)
	}
}

func TestContentRestrictionPropagatesMissingNestedModel(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	models := []contentModelNode{
		{Kind: modelSequence, Occurs: one, Particles: []contentParticle{modelParticle(99, one)}},
		{Kind: modelSequence, Occurs: one},
	}
	rt := contentRestrictionRuntimeWith(models)
	err := validateContentRestriction(rt.build(), 0, 1, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build()))
	if !isContentRestrictionInvariant(err) || !strings.Contains(err.Error(), "missing content model") {
		t.Fatalf("ValidateContentRestriction() error = %v, want missing-model invariant", err)
	}
}

func TestContentRestrictionRejectsRepeatedUnitChoiceBranchWithTail(t *testing.T) {
	t.Parallel()
	const (
		choiceID  contentModelID = 0
		baseID    contentModelID = 1
		derivedID contentModelID = 2
		elemA     ElementID      = 1
		elemB     ElementID      = 2
	)
	one := occurrence{Min: 1, Max: 1}
	repeat := occurrence{Min: 0, Unbounded: true}
	models := []contentModelNode{
		{Kind: modelChoice, Occurs: one, Particles: []contentParticle{elementParticle(elemA, one), elementParticle(elemB, one)}},
		{Kind: modelSequence, Occurs: one, Particles: []contentParticle{modelParticle(choiceID, repeat), elementParticle(elemB, one)}},
		{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(elemA, repeat), elementParticle(elemB, repeat)}},
	}
	rt := contentRestrictionRuntimeWith(models)
	if err := validateContentRestriction(rt.build(), baseID, derivedID, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt.build())); !isContentRestrictionMismatch(err) {
		t.Fatalf("ValidateContentRestriction() error = %v, want repeated element/unit branch mismatch", err)
	}
}

func contentRestrictionRuntimeWith(models []contentModelNode) contentRestrictionFixture {
	nameA := QName{Namespace: emptyNamespaceID, Local: 1}
	nameB := QName{Namespace: emptyNamespaceID, Local: 2}
	return contentRestrictionFixture{
		models:   models,
		elements: []QName{{}, nameA, nameB},
		elementRestrictions: []particleRestrictionElement{
			{},
			{Type: ComplexRef(0), Scope: declarationScopeNonGlobal},
			{Type: ComplexRef(0), Scope: declarationScopeNonGlobal},
		},
		anyType: 0,
	}
}

type contentRestrictionFixture struct {
	models              []contentModelNode
	elements            []QName
	elementRestrictions []particleRestrictionElement
	complexDerivations  []complexTypeDerivation
	simpleDerivations   []simpleTypeDerivation
	anyType             ComplexTypeID
}

func TestValidateParticleShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		in      contentParticle
	}{
		{
			name: "element constructor",
			in:   elementParticle(1, occurrence{Min: 1, Max: 1}),
		},
		{
			name: "model constructor",
			in:   modelParticle(2, occurrence{Min: 0, Max: 1}),
		},
		{
			name: "wildcard constructor",
			in:   wildcardParticle(3, occurrence{Min: 0, Unbounded: true}),
		},
		{
			name:    "invalid occurrence",
			in:      elementParticle(1, occurrence{Min: 1, Max: 1, Unbounded: true}),
			wantErr: "particle occurrence is invalid",
		},
		{
			name:    "invalid kind",
			in:      contentParticle{Kind: particleKind(99), Occurs: occurrence{Min: 1, Max: 1}},
			wantErr: "particle has invalid kind",
		},
		{
			name:    "element stores model",
			in:      contentParticle{Kind: particleElement, Element: 1, Model: 2, Wildcard: NoWildcard, Occurs: occurrence{Min: 1, Max: 1}},
			wantErr: "particle stores content model ID for non-model kind",
		},
		{
			name:    "model stores element",
			in:      contentParticle{Kind: particleModel, Element: 1, Model: 2, Wildcard: NoWildcard, Occurs: occurrence{Min: 1, Max: 1}},
			wantErr: "particle stores element ID for non-element kind",
		},
		{
			name:    "wildcard stores model",
			in:      contentParticle{Kind: particleWildcard, Element: NoElement, Model: 2, Wildcard: 3, Occurs: occurrence{Min: 1, Max: 1}},
			wantErr: "particle stores content model ID for non-model kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateParticleShape(tt.in)
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

	if !((occurrence{Min: 1, Max: 1}).IsExactlyOne()) {
		t.Fatal("1..1 occurrence should be exactly one")
	}
	for _, o := range []occurrence{
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
	b.typeWork = testDependencyWork()
	b.Elements = fixtureElements(f.elements, f.elementRestrictions)
	return b
}
