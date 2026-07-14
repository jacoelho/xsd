package runtime

import (
	"errors"
	"slices"
)

// ModelKind identifies the runtime content-model shape.
type ModelKind uint8

const (
	// ModelEmpty is an empty content model.
	ModelEmpty ModelKind = iota
	// ModelAny is the builtin xs:anyType content model.
	ModelAny
	// ModelSequence is an xs:sequence content model.
	ModelSequence
	// ModelChoice is an xs:choice content model.
	ModelChoice
	// ModelAll is an xs:all content model.
	ModelAll
)

// ParticleKind identifies the active reference in a Particle.
type ParticleKind uint8

const (
	// ParticleElement references an element declaration.
	ParticleElement ParticleKind = iota
	// ParticleModel references a nested content model.
	ParticleModel
	// ParticleWildcard references a wildcard.
	ParticleWildcard
)

// Occurrence stores min/max occurrence constraints.
type Occurrence struct {
	Min       uint32
	Max       uint32
	Unbounded bool
}

// IsExactlyOne reports whether the occurrence range is exactly 1..1.
func (o Occurrence) IsExactlyOne() bool {
	return o.Min == 1 && o.Max == 1 && !o.Unbounded
}

// ContentModel is a runtime content-model tree node.
type ContentModel struct {
	Particles    []Particle
	ChoiceLimits []uint32
	Occurs       Occurrence
	Kind         ModelKind
	Mixed        bool
}

// ContentModelByID resolves and clones a content model from a content-model
// table.
func ContentModelByID(models []ContentModel, id ContentModelID) (ContentModel, bool) {
	if !ValidContentModelID(id, len(models)) {
		return ContentModel{}, false
	}
	return CloneContentModel(models[id]), true
}

// Particle is a tagged union: Kind selects which ID field is active.
type Particle struct {
	Kind     ParticleKind
	Occurs   Occurrence
	Element  ElementID
	Model    ContentModelID
	Wildcard WildcardID
}

// ContentModelRefLimits are table sizes used to validate cross-table particle
// references in a frozen runtime schema.
type ContentModelRefLimits struct {
	ElementCount      int
	ContentModelCount int
	WildcardCount     int
}

// ParticleRestrictionElement is the element-declaration projection needed to
// evaluate particle restriction over frozen runtime metadata.
type ParticleRestrictionElement struct {
	Identities IdentityConstraintIDs
	Fixed      ValueConstraintIdentity
	Type       TypeID
	Block      DerivationMask
	Scope      DeclarationScope
	Nillable   bool
}

// ParticleRestrictionRuntime supplies read-only runtime metadata needed to
// evaluate particle restriction.
type ParticleRestrictionRuntime interface {
	ParticleRuntime
	TypeDerivationRuntime
	ElementRestriction(id ElementID) (ParticleRestrictionElement, bool)
}

// ElementParticle returns an element particle with inactive fields pinned.
func ElementParticle(id ElementID, occurs Occurrence) Particle {
	return Particle{Kind: ParticleElement, Occurs: occurs, Element: id, Model: NoContentModel, Wildcard: NoWildcard}
}

// ModelParticle returns a model particle with inactive fields pinned.
func ModelParticle(id ContentModelID, occurs Occurrence) Particle {
	return Particle{Kind: ParticleModel, Occurs: occurs, Element: NoElement, Model: id, Wildcard: NoWildcard}
}

// WildcardParticle returns a wildcard particle with inactive fields pinned.
func WildcardParticle(id WildcardID, occurs Occurrence) Particle {
	return Particle{Kind: ParticleWildcard, Occurs: occurs, Element: NoElement, Model: NoContentModel, Wildcard: id}
}

// ValidateContentModelShape validates content-model metadata that does not
// require cross-table ID lookup.
func ValidateContentModelShape(model ContentModel) error {
	switch model.Kind {
	case ModelEmpty:
		if len(model.Particles) != 0 || len(model.ChoiceLimits) != 0 || model.Occurs != (Occurrence{}) {
			return errors.New("empty content model stores inactive fields")
		}
	case ModelAny:
		if len(model.Particles) != 0 || len(model.ChoiceLimits) != 0 || model.Occurs != (Occurrence{}) || !model.Mixed {
			return errors.New("any content model has invalid shape")
		}
	case ModelSequence, ModelChoice:
		if !validOccurrence(model.Occurs) {
			return errors.New("content model occurrence is invalid")
		}
		if model.Kind != ModelSequence && len(model.ChoiceLimits) != 0 {
			return errors.New("non-sequence content model stores choice limits")
		}
		if err := validateChoiceLimits(model); err != nil {
			return err
		}
	case ModelAll:
		if !validOccurrence(model.Occurs) || model.Occurs.Unbounded || model.Occurs.Max > 1 || model.Occurs.Min > 1 {
			return errors.New("all content model occurrence is invalid")
		}
		if len(model.ChoiceLimits) != 0 {
			return errors.New("all content model stores choice limits")
		}
	default:
		return errors.New("content model has invalid kind")
	}
	for _, p := range model.Particles {
		if err := ValidateParticleShape(p); err != nil {
			return err
		}
	}
	return nil
}

// ValidateContentModelRuntime validates content-model shape and cross-table
// particle references.
func ValidateContentModelRuntime(model ContentModel, limits ContentModelRefLimits) error {
	if err := ValidateContentModelShape(model); err != nil {
		return err
	}
	for _, p := range model.Particles {
		switch p.Kind {
		case ParticleElement:
			if !ValidElementID(p.Element, limits.ElementCount) {
				return errors.New("particle references invalid element")
			}
		case ParticleModel:
			if !ValidContentModelID(p.Model, limits.ContentModelCount) {
				return errors.New("particle references invalid content model")
			}
		case ParticleWildcard:
			if !ValidWildcardID(p.Wildcard, limits.WildcardCount) {
				return errors.New("particle references invalid wildcard")
			}
		default:
			return errors.New("particle has invalid kind")
		}
	}
	return nil
}

type contentModelGraphState uint8

const (
	contentModelGraphUnchecked contentModelGraphState = iota
	contentModelGraphChecking
	contentModelGraphChecked
)

type contentModelGraphFrame struct {
	id   ContentModelID
	next int
}

func validateContentModelGraph(models []ContentModel) error {
	state := make([]contentModelGraphState, len(models))
	stack := make([]contentModelGraphFrame, 0, min(len(models), 1_024))
	for i := range models {
		root := ContentModelID(i)
		if state[root] == contentModelGraphChecked {
			continue
		}
		state[root] = contentModelGraphChecking
		stack = appendDFSFrame(stack, contentModelGraphFrame{id: root}, len(models))
		for len(stack) != 0 {
			top := len(stack) - 1
			frame := &stack[top]
			particles := models[frame.id].Particles
			for frame.next < len(particles) && particles[frame.next].Kind != ParticleModel {
				frame.next++
			}
			if frame.next == len(particles) {
				state[frame.id] = contentModelGraphChecked
				stack = stack[:top]
				continue
			}
			child := particles[frame.next].Model
			frame.next++
			if !ValidContentModelID(child, len(models)) {
				return errors.New("content model graph references invalid model")
			}
			switch state[child] {
			case contentModelGraphUnchecked:
				state[child] = contentModelGraphChecking
				stack = appendDFSFrame(stack, contentModelGraphFrame{id: child}, len(models))
			case contentModelGraphChecking:
				return errors.New("content model graph contains cycle")
			case contentModelGraphChecked:
			}
		}
	}
	return nil
}

// ComplexContentExtendsBase reports whether derived preserves base as the
// leading content of a complex-type extension.
func ComplexContentExtendsBase(rt ContentModelRuntime, baseID, derivedID ContentModelID) bool {
	if baseID == derivedID || ModelHasNoParticles(rt, baseID) {
		return true
	}
	base, ok := rt.ContentModel(baseID)
	if !ok {
		return false
	}
	derived, ok := rt.ContentModel(derivedID)
	if !ok {
		return false
	}
	if derived.Kind != ModelSequence {
		return false
	}
	if !derived.Occurs.IsExactlyOne() {
		return false
	}
	if base.Kind == ModelSequence && base.Occurs.IsExactlyOne() {
		return len(derived.Particles) >= len(base.Particles) &&
			slices.Equal(derived.Particles[:len(base.Particles)], base.Particles)
	}
	return len(derived.Particles) != 0 &&
		derived.Particles[0] == ModelParticle(baseID, Occurrence{Min: 1, Max: 1})
}

func validateChoiceLimits(model ContentModel) error {
	if len(model.ChoiceLimits) == 0 {
		return nil
	}
	if model.Kind != ModelSequence {
		return errors.New("choice limits require sequence content model")
	}
	var prev uint32
	for i, slot := range model.ChoiceLimits {
		if !ValidUint32Index(slot, len(model.Particles)) {
			return errors.New("choice limit references invalid particle")
		}
		if i != 0 && slot <= prev {
			return errors.New("choice limits are not sorted")
		}
		p := model.Particles[slot]
		if p.Kind != ParticleElement || p.Occurs.Min > 1 || (!p.Occurs.Unbounded && p.Occurs.Max <= 1) {
			return errors.New("choice limit references invalid particle shape")
		}
		prev = slot
	}
	return nil
}

// RestrictionRepeatedChoiceParticles derives the derived sequence particle
// slots that must be limited to one match because they restrict a repeated
// base choice.
func RestrictionRepeatedChoiceParticles(
	models []ContentModel,
	baseID, derivedID ContentModelID,
	rt ParticleRestrictionRuntime,
) ([]uint32, error) {
	if rt == nil {
		return nil, errors.New("choice-limit derivation requires runtime")
	}
	if !ValidContentModelID(baseID, len(models)) ||
		!ValidContentModelID(derivedID, len(models)) {
		return nil, errors.New("choice-limit derivation references invalid content model")
	}
	base := models[baseID]
	derived := models[derivedID]
	if base.Kind != ModelSequence || derived.Kind != ModelSequence {
		return nil, nil
	}
	var out []uint32
	validator := contentRestrictionValidator{rt: rt, modelStates: make(map[ContentModelID]uint8)}
	if err := validator.validateContentModelGraph(baseID); err != nil {
		return nil, err
	}
	if err := validator.validateContentModelGraph(derivedID); err != nil {
		return nil, err
	}
	baseIndex := 0
	for derivedIndex, derivedParticle := range derived.Particles {
		for baseIndex < len(base.Particles) {
			baseParticle := base.Particles[baseIndex]
			err := validator.validateParticleRestriction(baseParticle, derivedParticle)
			if err != nil {
				if !isContentRestrictionMismatch(err) {
					return nil, err
				}
				baseIndex++
				continue
			}
			if restrictionRepeatedChoiceParticle(models, baseParticle, derivedParticle) {
				if uint64(derivedIndex) > uint64(^uint32(0)) {
					return nil, errors.New("choice-limit particle index exceeds uint32")
				}
				out = append(out, uint32(derivedIndex))
			}
			baseIndex++
			break
		}
	}
	return out, nil
}

func restrictionRepeatedChoiceParticle(models []ContentModel, baseParticle, derivedParticle Particle) bool {
	if baseParticle.Kind != ParticleModel || baseParticle.Occurs.IsExactlyOne() {
		return false
	}
	if !ValidContentModelID(baseParticle.Model, len(models)) {
		return false
	}
	model := models[baseParticle.Model]
	if model.Kind != ModelChoice || derivedParticle.Kind != ParticleElement {
		return false
	}
	return derivedParticle.Occurs.Min <= 1 && derivedParticle.Occurs.Unbounded
}

// RestrictionChoiceLimitUpdate is a content-model copy that must be assigned to
// one restricting complex type so repeated-choice limits stay owner-private.
type RestrictionChoiceLimitUpdate struct {
	Model       ContentModel
	ComplexType ComplexTypeID
}

// RestrictionChoiceLimitUpdates derives owner-private content-model copies for
// restricting complex types whose particles need repeated-choice limits.
func RestrictionChoiceLimitUpdates(
	rt ParticleRestrictionRuntime,
	complexTypes []ComplexType,
	models []ContentModel,
	anyType ComplexTypeID,
) ([]RestrictionChoiceLimitUpdate, error) {
	if rt == nil {
		return nil, errors.New("choice-limit derivation requires runtime")
	}
	var updates []RestrictionChoiceLimitUpdate
	for i, ct := range complexTypes {
		if uint64(i) >= uint64(invalidID) {
			return nil, errors.New("complex type index limit exceeded")
		}
		if ct.Derivation != DerivationKindRestriction {
			continue
		}
		baseID, ok := ct.Base.Complex()
		if !ok || baseID == anyType {
			continue
		}
		if !ValidComplexTypeID(baseID, len(complexTypes)) {
			return nil, errors.New("choice-limit restriction references invalid base complex type")
		}
		if !ValidContentModelID(ct.Content, len(models)) {
			return nil, errors.New("choice-limit restriction references invalid derived content model")
		}
		baseContent := complexTypes[baseID].Content
		if !ValidContentModelID(baseContent, len(models)) {
			return nil, errors.New("choice-limit restriction references invalid base content model")
		}
		repeated, err := RestrictionRepeatedChoiceParticles(models, baseContent, ct.Content, rt)
		if err != nil {
			return nil, err
		}
		if len(repeated) == 0 {
			continue
		}
		model := CloneContentModel(models[ct.Content])
		if len(model.ChoiceLimits) != 0 && !slices.Equal(model.ChoiceLimits, repeated) {
			return nil, errors.New("choice-limit restriction source model already has different choice limits")
		}
		model.ChoiceLimits = slices.Clone(repeated)
		updates = append(updates, RestrictionChoiceLimitUpdate{
			Model:       model,
			ComplexType: ComplexTypeID(i),
		})
	}
	return updates, nil
}

// ValidateChoiceLimitDerivations validates that every ContentModel.ChoiceLimits
// entry is exactly justified by restricting complex-type derivations, and that
// limited content models are not shared outside those owners.
func ValidateChoiceLimitDerivations(
	rt ParticleRestrictionRuntime,
	complexTypes []ComplexType,
	models []ContentModel,
	anyType ComplexTypeID,
) error {
	if rt == nil {
		return errors.New("choice-limit validation requires runtime")
	}
	expected := make(map[ContentModelID][]uint32)
	owners := make(map[ContentModelID][]ComplexTypeID)
	for i, ct := range complexTypes {
		if uint64(i) >= uint64(invalidID) {
			return errors.New("complex type index limit exceeded")
		}
		if !ValidContentModelID(ct.Content, len(models)) {
			continue
		}
		id := ComplexTypeID(i)
		owners[ct.Content] = append(owners[ct.Content], id)
		if ct.Derivation != DerivationKindRestriction {
			continue
		}
		baseID, ok := ct.Base.Complex()
		if !ok || baseID == anyType || !ValidComplexTypeID(baseID, len(complexTypes)) {
			continue
		}
		repeated, err := RestrictionRepeatedChoiceParticles(models, complexTypes[baseID].Content, ct.Content, rt)
		if err != nil {
			return err
		}
		if len(repeated) == 0 {
			continue
		}
		if prev, ok := expected[ct.Content]; ok && !slices.Equal(prev, repeated) {
			return errors.New("content model choice limits have conflicting derivations")
		}
		expected[ct.Content] = repeated
	}
	for i, model := range models {
		if uint64(i) >= uint64(invalidID) {
			return errors.New("content model index limit exceeded")
		}
		id := ContentModelID(i)
		if !slices.Equal(model.ChoiceLimits, expected[id]) {
			return errors.New("content model choice limits do not match complex restrictions")
		}
		if len(model.ChoiceLimits) == 0 {
			continue
		}
		for _, ownerID := range owners[id] {
			if !ValidComplexTypeID(ownerID, len(complexTypes)) {
				return errors.New("limited content model has invalid restriction owner")
			}
			owner := complexTypes[ownerID]
			if owner.Derivation != DerivationKindRestriction {
				return errors.New("limited content model is used outside restricting complex type")
			}
			baseID, ok := owner.Base.Complex()
			if !ok || baseID == anyType || !ValidComplexTypeID(baseID, len(complexTypes)) {
				return errors.New("limited content model has invalid restriction owner")
			}
			repeated, err := RestrictionRepeatedChoiceParticles(models, complexTypes[baseID].Content, owner.Content, rt)
			if err != nil {
				return err
			}
			if !slices.Equal(repeated, model.ChoiceLimits) {
				return errors.New("limited content model owner does not derive choice limits")
			}
		}
	}
	return nil
}

func validOccurrence(o Occurrence) bool {
	if o.Unbounded {
		return o.Max == 0
	}
	return o.Max >= o.Min
}

// ValidateParticleShape validates particle metadata that does not require
// cross-table ID lookup.
func ValidateParticleShape(p Particle) error {
	if !validOccurrence(p.Occurs) {
		return errors.New("particle occurrence is invalid")
	}
	switch p.Kind {
	case ParticleElement, ParticleModel, ParticleWildcard:
	default:
		return errors.New("particle has invalid kind")
	}
	if p.Kind != ParticleElement && p.Element != NoElement {
		return errors.New("particle stores element ID for non-element kind")
	}
	if p.Kind != ParticleModel && p.Model != NoContentModel {
		return errors.New("particle stores content model ID for non-model kind")
	}
	if p.Kind != ParticleWildcard && p.Wildcard != NoWildcard {
		return errors.New("particle stores wildcard ID for non-wildcard kind")
	}
	return nil
}
