package runtime

import (
	"slices"
	"strings"
	"testing"
)

func TestContentModelByID(t *testing.T) {
	t.Parallel()

	models := []ContentModel{
		{Kind: ModelEmpty},
		{
			Kind:         ModelSequence,
			Occurs:       Occurrence{Min: 1, Max: 1},
			Particles:    []Particle{ElementParticle(1, Occurrence{Min: 1, Max: 1})},
			ChoiceLimits: []uint32{0},
		},
	}
	got, ok := ContentModelByID(models, 1)
	if !ok ||
		got.Kind != ModelSequence ||
		!slices.Equal(got.Particles, models[1].Particles) ||
		!slices.Equal(got.ChoiceLimits, models[1].ChoiceLimits) {
		t.Fatalf("ContentModelByID(valid) = %+v, %v; want cloned model 1, true", got, ok)
	}
	got.Particles[0].Element = 9
	got.ChoiceLimits[0] = 9
	if models[1].Particles[0].Element != 1 || models[1].ChoiceLimits[0] != 0 {
		t.Fatalf("ContentModelByID aliased slice fields: %+v", models[1])
	}
	for _, id := range []ContentModelID{NoContentModel, 2} {
		got, ok := ContentModelByID(models, id)
		if ok || got.Kind != 0 || len(got.Particles) != 0 || len(got.ChoiceLimits) != 0 {
			t.Fatalf("ContentModelByID(%d) = %+v, %v; want zero, false", id, got, ok)
		}
	}
}

func TestValidateContentModelShape(t *testing.T) {
	t.Parallel()

	repeatingElement := ElementParticle(1, Occurrence{Min: 0, Max: 2})
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
			name: "sequence choice limits",
			in: ContentModel{
				Kind:         ModelSequence,
				Occurs:       Occurrence{Min: 1, Max: 1},
				Particles:    []Particle{repeatingElement},
				ChoiceLimits: []uint32{0},
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
			name: "choice stores choice limits",
			in: ContentModel{
				Kind:         ModelChoice,
				Occurs:       Occurrence{Min: 1, Max: 1},
				Particles:    []Particle{repeatingElement},
				ChoiceLimits: []uint32{0},
			},
			wantErr: "non-sequence content model stores choice limits",
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
			name: "choice limit out of range",
			in: ContentModel{
				Kind:         ModelSequence,
				Occurs:       Occurrence{Min: 1, Max: 1},
				Particles:    []Particle{repeatingElement},
				ChoiceLimits: []uint32{1},
			},
			wantErr: "choice limit references invalid particle",
		},
		{
			name: "choice limits unsorted",
			in: ContentModel{
				Kind:         ModelSequence,
				Occurs:       Occurrence{Min: 1, Max: 1},
				Particles:    []Particle{repeatingElement, ElementParticle(2, Occurrence{Min: 0, Max: 2})},
				ChoiceLimits: []uint32{1, 0},
			},
			wantErr: "choice limits are not sorted",
		},
		{
			name: "choice limits duplicate",
			in: ContentModel{
				Kind:         ModelSequence,
				Occurs:       Occurrence{Min: 1, Max: 1},
				Particles:    []Particle{repeatingElement},
				ChoiceLimits: []uint32{0, 0},
			},
			wantErr: "choice limits are not sorted",
		},
		{
			name: "choice limit requires repeating element",
			in: ContentModel{
				Kind:         ModelSequence,
				Occurs:       Occurrence{Min: 1, Max: 1},
				Particles:    []Particle{ElementParticle(1, Occurrence{Min: 1, Max: 1})},
				ChoiceLimits: []uint32{0},
			},
			wantErr: "choice limit references invalid particle shape",
		},
		{
			name: "choice limit rejects model particle",
			in: ContentModel{
				Kind:         ModelSequence,
				Occurs:       Occurrence{Min: 1, Max: 1},
				Particles:    []Particle{ModelParticle(1, Occurrence{Min: 0, Max: 2})},
				ChoiceLimits: []uint32{0},
			},
			wantErr: "choice limit references invalid particle shape",
		},
		{
			name: "choice limit rejects wildcard particle",
			in: ContentModel{
				Kind:         ModelSequence,
				Occurs:       Occurrence{Min: 1, Max: 1},
				Particles:    []Particle{WildcardParticle(1, Occurrence{Min: 0, Max: 2})},
				ChoiceLimits: []uint32{0},
			},
			wantErr: "choice limit references invalid particle shape",
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
	rt := testParticleRuntime{
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

			if got := ComplexContentExtendsBase(rt, tt.base, tt.derived); got != tt.want {
				t.Fatalf("ComplexContentExtendsBase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRestrictionRepeatedChoiceParticles(t *testing.T) {
	t.Parallel()

	const (
		choiceID  ContentModelID = 0
		baseID    ContentModelID = 1
		derivedID ContentModelID = 2
	)
	one := Occurrence{Min: 1, Max: 1}
	repeat := Occurrence{Min: 0, Unbounded: true}
	models := []ContentModel{
		{
			Kind:      ModelChoice,
			Occurs:    one,
			Particles: []Particle{ElementParticle(1, one), ElementParticle(2, one)},
		},
		{
			Kind:      ModelSequence,
			Occurs:    one,
			Particles: []Particle{ModelParticle(choiceID, repeat)},
		},
		{
			Kind:      ModelSequence,
			Occurs:    one,
			Particles: []Particle{ElementParticle(1, repeat)},
		},
	}
	rt := choiceLimitRuntimeWith(models)
	got := RestrictionRepeatedChoiceParticles(models, baseID, derivedID, rt)
	if !slices.Equal(got, []uint32{0}) {
		t.Fatalf("RestrictionRepeatedChoiceParticles() = %v, want [0]", got)
	}
	models[baseID].Particles[0].Occurs = one
	rt = choiceLimitRuntimeWith(models)
	if got := RestrictionRepeatedChoiceParticles(models, baseID, derivedID, rt); len(got) != 0 {
		t.Fatalf("RestrictionRepeatedChoiceParticles() with exact-one base = %v, want nil", got)
	}
	models[baseID].Particles[0].Occurs = repeat
	models[derivedID].Particles[0].Occurs = one
	rt = choiceLimitRuntimeWith(models)
	if got := RestrictionRepeatedChoiceParticles(models, baseID, derivedID, rt); len(got) != 0 {
		t.Fatalf("RestrictionRepeatedChoiceParticles() with non-repeating derived = %v, want nil", got)
	}
}

func TestRestrictionChoiceLimitUpdates(t *testing.T) {
	t.Parallel()

	const (
		anyType     ComplexTypeID  = 0
		baseType    ComplexTypeID  = 1
		derivedType ComplexTypeID  = 2
		choiceID    ContentModelID = 0
		baseID      ContentModelID = 1
		derivedID   ContentModelID = 2
		baseElem    ElementID      = 0
		derivedElem ElementID      = 1
	)
	one := Occurrence{Min: 1, Max: 1}
	repeat := Occurrence{Min: 0, Unbounded: true}
	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	models := []ContentModel{
		{
			Kind:      ModelChoice,
			Occurs:    one,
			Particles: []Particle{ElementParticle(baseElem, one)},
		},
		{
			Kind:      ModelSequence,
			Occurs:    one,
			Particles: []Particle{ModelParticle(choiceID, repeat)},
		},
		{
			Kind:      ModelSequence,
			Occurs:    one,
			Particles: []Particle{ElementParticle(derivedElem, repeat)},
		},
	}
	complexTypes := []ComplexType{
		{Content: NoContentModel},
		{Content: baseID},
		{Base: ComplexRef(baseType), Content: derivedID, Derivation: DerivationKindRestriction},
	}
	rt := choiceLimitRestrictionRuntime{
		models:   models,
		elements: []QName{name, name},
		elementRestrictions: []ParticleRestrictionElement{
			{Type: ComplexRef(anyType)},
			{Type: ComplexRef(anyType)},
		},
		anyType: anyType,
	}

	updates, err := RestrictionChoiceLimitUpdates(rt, complexTypes, models, anyType)
	if err != nil {
		t.Fatalf("RestrictionChoiceLimitUpdates() error = %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("RestrictionChoiceLimitUpdates() returned %d updates, want 1", len(updates))
	}
	if updates[0].ComplexType != derivedType {
		t.Fatalf("update complex type = %d, want %d", updates[0].ComplexType, derivedType)
	}
	if !slices.Equal(updates[0].Model.ChoiceLimits, []uint32{0}) {
		t.Fatalf("update choice limits = %v, want [0]", updates[0].Model.ChoiceLimits)
	}
	if len(models[derivedID].ChoiceLimits) != 0 {
		t.Fatalf("source model choice limits = %v, want nil", models[derivedID].ChoiceLimits)
	}
	updates[0].Model.ChoiceLimits[0] = 9
	if len(models[derivedID].ChoiceLimits) != 0 {
		t.Fatalf("mutating update changed source choice limits to %v", models[derivedID].ChoiceLimits)
	}
	updates[0].Model.Particles[0].Element = 9
	if models[derivedID].Particles[0].Element != derivedElem {
		t.Fatalf("mutating update changed source particles to %v", models[derivedID].Particles)
	}
}

func TestValidateChoiceLimitDerivationsUsesRuntimeParticleRestriction(t *testing.T) {
	t.Parallel()

	const (
		anyType     ComplexTypeID  = 0
		baseType    ComplexTypeID  = 1
		derivedType ComplexTypeID  = 2
		choiceID    ContentModelID = 0
		baseID      ContentModelID = 1
		derivedID   ContentModelID = 2
		baseElem    ElementID      = 0
		derivedElem ElementID      = 1
	)
	one := Occurrence{Min: 1, Max: 1}
	repeat := Occurrence{Min: 0, Unbounded: true}
	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	complexTypes := []ComplexType{
		{Content: NoContentModel},
		{Content: baseID},
		{Base: ComplexRef(baseType), Content: derivedID, Derivation: DerivationKindRestriction},
	}
	rt := choiceLimitRestrictionRuntime{
		models: []ContentModel{
			{
				Kind:      ModelChoice,
				Occurs:    one,
				Particles: []Particle{ElementParticle(baseElem, one)},
			},
			{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{ModelParticle(choiceID, repeat)},
			},
			{
				Kind:         ModelSequence,
				Occurs:       one,
				Particles:    []Particle{ElementParticle(derivedElem, repeat)},
				ChoiceLimits: []uint32{0},
			},
		},
		elements: []QName{name, name},
		elementRestrictions: []ParticleRestrictionElement{
			{Type: ComplexRef(anyType)},
			{Type: ComplexRef(anyType)},
		},
		anyType: anyType,
	}
	if err := ValidateChoiceLimitDerivations(rt, complexTypes, rt.models, anyType); err != nil {
		t.Fatalf("ValidateChoiceLimitDerivations() error = %v", err)
	}

	rt.elements[derivedElem] = QName{Namespace: EmptyNamespaceID, Local: 2}
	if err := ValidateChoiceLimitDerivations(rt, complexTypes, rt.models, anyType); err == nil ||
		!strings.Contains(err.Error(), "content model choice limits do not match complex restrictions") {
		t.Fatalf("ValidateChoiceLimitDerivations() error = %v, want choice-limit mismatch", err)
	}
}

func TestParticleRestrictsUsesFixedValueIdentity(t *testing.T) {
	t.Parallel()

	const (
		baseType    SimpleTypeID = 1
		derivedType SimpleTypeID = 2
	)
	one := Occurrence{Min: 1, Max: 1}
	name := QName{Namespace: EmptyNamespaceID, Local: 1}
	identity := SimpleIdentityKey(PrimitiveDecimal, "5")
	rt := choiceLimitRestrictionRuntime{
		elements: []QName{name, name},
		elementRestrictions: []ParticleRestrictionElement{
			{Type: SimpleRef(baseType), Fixed: fixedValueConstraintIdentity("5.0", "5.0", baseType, identity)},
			{Type: SimpleRef(derivedType), Fixed: fixedValueConstraintIdentity("5", "5", derivedType, identity)},
		},
		simpleDerivations: []SimpleTypeDerivation{
			{},
			{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
			{Base: baseType, Variety: SimpleVarietyAtomic},
		},
	}
	if !ParticleRestricts(rt, ElementParticle(0, one), ElementParticle(1, one)) {
		t.Fatal("ParticleRestricts() rejected equal fixed value identities")
	}

	rt.elementRestrictions[1].Fixed.Value.Identity = SimpleIdentityKey(PrimitiveString, "5")
	if ParticleRestricts(rt, ElementParticle(0, one), ElementParticle(1, one)) {
		t.Fatal("ParticleRestricts() accepted different fixed value identities")
	}
}

func TestValidateChoiceLimitDerivations(t *testing.T) {
	t.Parallel()

	const (
		anyType     ComplexTypeID  = 0
		baseType    ComplexTypeID  = 1
		derivedType ComplexTypeID  = 2
		base2Type   ComplexTypeID  = 3
		choiceID    ContentModelID = 0
		baseID      ContentModelID = 1
		derivedID   ContentModelID = 2
		base2ID     ContentModelID = 3
		elemA       ElementID      = 1
		elemB       ElementID      = 2
	)
	one := Occurrence{Min: 1, Max: 1}
	repeat := Occurrence{Min: 0, Unbounded: true}
	models := []ContentModel{
		{
			Kind:      ModelChoice,
			Occurs:    one,
			Particles: []Particle{ElementParticle(elemA, one), ElementParticle(elemB, one)},
		},
		{
			Kind: ModelSequence,
			Particles: []Particle{
				ModelParticle(choiceID, repeat),
				ElementParticle(elemB, one),
			},
			Occurs: one,
		},
		{
			Kind: ModelSequence,
			Particles: []Particle{
				ElementParticle(elemA, repeat),
				ElementParticle(elemB, repeat),
			},
			ChoiceLimits: []uint32{0},
			Occurs:       one,
		},
		{
			Kind: ModelSequence,
			Particles: []Particle{
				ElementParticle(elemA, repeat),
				ModelParticle(choiceID, repeat),
			},
			Occurs: one,
		},
	}
	complexTypes := []ComplexType{
		{Content: NoContentModel},
		{Content: baseID},
		{Base: ComplexRef(baseType), Content: derivedID, Derivation: DerivationKindRestriction},
	}
	tests := []struct {
		name         string
		wantErr      string
		complexTypes []ComplexType
		models       []ContentModel
	}{
		{
			name:         "valid restriction limits",
			complexTypes: complexTypes,
			models:       models,
		},
		{
			name:         "missing derived limits",
			complexTypes: complexTypes,
			models:       choiceLimitModelsWith(models, derivedID, nil),
			wantErr:      "content model choice limits do not match complex restrictions",
		},
		{
			name: "limited model used by non-restriction",
			complexTypes: append(slices.Clone(complexTypes), ComplexType{
				Base:       ComplexRef(baseType),
				Content:    derivedID,
				Derivation: DerivationKindExtension,
			}),
			models:  models,
			wantErr: "limited content model is used outside restricting complex type",
		},
		{
			name: "limited model has invalid restriction owner",
			complexTypes: append(slices.Clone(complexTypes), ComplexType{
				Base:       ComplexRef(anyType),
				Content:    derivedID,
				Derivation: DerivationKindRestriction,
			}),
			models:  models,
			wantErr: "limited content model has invalid restriction owner",
		},
		{
			name: "conflicting derivations",
			complexTypes: append(slices.Clone(complexTypes),
				ComplexType{Content: base2ID},
				ComplexType{Base: ComplexRef(base2Type), Content: derivedID, Derivation: DerivationKindRestriction},
			),
			models:  models,
			wantErr: "content model choice limits have conflicting derivations",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateChoiceLimitDerivations(choiceLimitRuntimeWith(tt.models), tt.complexTypes, tt.models, anyType)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateChoiceLimitDerivations() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateChoiceLimitDerivations() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func choiceLimitModelsWith(models []ContentModel, id ContentModelID, limits []uint32) []ContentModel {
	out := slices.Clone(models)
	out[id].ChoiceLimits = limits
	return out
}

func choiceLimitRuntimeWith(models []ContentModel) choiceLimitRestrictionRuntime {
	nameA := QName{Namespace: EmptyNamespaceID, Local: 1}
	nameB := QName{Namespace: EmptyNamespaceID, Local: 2}
	return choiceLimitRestrictionRuntime{
		models:   models,
		elements: []QName{{}, nameA, nameB},
		elementRestrictions: []ParticleRestrictionElement{
			{},
			{Type: ComplexRef(0)},
			{Type: ComplexRef(0)},
		},
		anyType: 0,
	}
}

type choiceLimitRestrictionRuntime struct {
	models              []ContentModel
	elements            []QName
	elementRestrictions []ParticleRestrictionElement
	complexDerivations  []ComplexTypeDerivation
	simpleDerivations   []SimpleTypeDerivation
	anyType             ComplexTypeID
}

func (rt choiceLimitRestrictionRuntime) ContentModel(id ContentModelID) (ContentModel, bool) {
	if !ValidUint32Index(uint32(id), len(rt.models)) {
		return ContentModel{}, false
	}
	return rt.models[id], true
}

func (rt choiceLimitRestrictionRuntime) ElementName(id ElementID) (QName, bool) {
	if !ValidUint32Index(uint32(id), len(rt.elements)) {
		return QName{}, false
	}
	return rt.elements[id], true
}

func (rt choiceLimitRestrictionRuntime) Wildcard(WildcardID) (Wildcard, bool) {
	return Wildcard{}, false
}

func (rt choiceLimitRestrictionRuntime) ForEachSubstitutionMember(ElementID, func(ElementID) bool) {
}

func (rt choiceLimitRestrictionRuntime) SubstitutionMemberByName(ElementID, QName) (ElementID, bool) {
	return NoElement, false
}

func (rt choiceLimitRestrictionRuntime) AnyTypeID() ComplexTypeID {
	return rt.anyType
}

func (rt choiceLimitRestrictionRuntime) ComplexTypeCount() int {
	return len(rt.complexDerivations)
}

func (rt choiceLimitRestrictionRuntime) SimpleTypeCount() int {
	return len(rt.simpleDerivations)
}

func (rt choiceLimitRestrictionRuntime) SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	if !ValidUint32Index(uint32(id), len(rt.simpleDerivations)) {
		return SimpleTypeDerivation{}, false
	}
	return rt.simpleDerivations[id], true
}

func (rt choiceLimitRestrictionRuntime) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	if !ValidUint32Index(uint32(id), len(rt.complexDerivations)) {
		return ComplexTypeDerivation{}, false
	}
	return rt.complexDerivations[id], true
}

func (rt choiceLimitRestrictionRuntime) ElementRestriction(id ElementID) (ParticleRestrictionElement, bool) {
	if !ValidUint32Index(uint32(id), len(rt.elementRestrictions)) {
		return ParticleRestrictionElement{}, false
	}
	return rt.elementRestrictions[id], true
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
