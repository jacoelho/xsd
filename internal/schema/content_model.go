package schema

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
	Particles []Particle
	Occurs    Occurrence
	Kind      ModelKind
	Mixed     bool
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
	if err := validateContentModelKindShape(model); err != nil {
		return err
	}
	for _, particle := range model.Particles {
		if err := ValidateParticleShape(particle); err != nil {
			return err
		}
	}
	return nil
}

func validateContentModelKindShape(model ContentModel) error {
	switch model.Kind {
	case ModelEmpty:
		return validateEmptyContentModelShape(model)
	case ModelAny:
		return validateAnyContentModelShape(model)
	case ModelSequence, ModelChoice:
		return validateCompositorContentModelShape(model)
	case ModelAll:
		return validateAllContentModelShape(model)
	default:
		return errors.New("content model has invalid kind")
	}
}

func validateEmptyContentModelShape(model ContentModel) error {
	if len(model.Particles) != 0 || model.Occurs != (Occurrence{}) {
		return errors.New("empty content model stores inactive fields")
	}
	return nil
}

func validateAnyContentModelShape(model ContentModel) error {
	if len(model.Particles) != 0 || model.Occurs != (Occurrence{}) || !model.Mixed {
		return errors.New("any content model has invalid shape")
	}
	return nil
}

func validateCompositorContentModelShape(model ContentModel) error {
	if !validOccurrence(model.Occurs) {
		return errors.New("content model occurrence is invalid")
	}
	return nil
}

func validateAllContentModelShape(model ContentModel) error {
	if !validOccurrence(model.Occurs) || model.Occurs.Unbounded || model.Occurs.Max > 1 || model.Occurs.Min > 1 {
		return errors.New("all content model occurrence is invalid")
	}
	return nil
}

// ValidateContentModelRuntime validates content-model shape and cross-table
// particle references.
func ValidateContentModelRuntime(model ContentModel, limits ContentModelRefLimits) error {
	if err := ValidateContentModelShape(model); err != nil {
		return err
	}
	for _, particle := range model.Particles {
		if err := validateParticleRuntimeReference(particle, limits); err != nil {
			return err
		}
	}
	return nil
}

func validateParticleRuntimeReference(particle Particle, limits ContentModelRefLimits) error {
	switch particle.Kind {
	case ParticleElement:
		if !ValidElementID(particle.Element, limits.ElementCount) {
			return errors.New("particle references invalid element")
		}
	case ParticleModel:
		if !ValidContentModelID(particle.Model, limits.ContentModelCount) {
			return errors.New("particle references invalid content model")
		}
	case ParticleWildcard:
		if !ValidWildcardID(particle.Wildcard, limits.WildcardCount) {
			return errors.New("particle references invalid wildcard")
		}
	default:
		return errors.New("particle has invalid kind")
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
	audit := contentModelGraphAudit{
		models: models,
		state:  make([]contentModelGraphState, len(models)),
		stack:  make([]contentModelGraphFrame, 0, min(len(models), 1_024)),
	}
	for i := range models {
		root := ContentModelID(i)
		if audit.state[root] == contentModelGraphChecked {
			continue
		}
		if err := audit.validateRoot(root); err != nil {
			return err
		}
	}
	return nil
}

type contentModelGraphAudit struct {
	models []ContentModel
	state  []contentModelGraphState
	stack  []contentModelGraphFrame
}

func (a *contentModelGraphAudit) validateRoot(root ContentModelID) error {
	a.state[root] = contentModelGraphChecking
	a.stack = appendDFSFrame(a.stack, contentModelGraphFrame{id: root}, len(a.models))
	for len(a.stack) != 0 {
		if err := a.advance(); err != nil {
			return err
		}
	}
	return nil
}

func (a *contentModelGraphAudit) advance() error {
	top := len(a.stack) - 1
	frame := &a.stack[top]
	child, ok := nextNestedContentModel(a.models[frame.id].Particles, &frame.next)
	if !ok {
		a.state[frame.id] = contentModelGraphChecked
		a.stack = a.stack[:top]
		return nil
	}
	if !ValidContentModelID(child, len(a.models)) {
		return errors.New("content model graph references invalid model")
	}
	switch a.state[child] {
	case contentModelGraphUnchecked:
		a.state[child] = contentModelGraphChecking
		a.stack = appendDFSFrame(a.stack, contentModelGraphFrame{id: child}, len(a.models))
	case contentModelGraphChecking:
		return errors.New("content model graph contains cycle")
	case contentModelGraphChecked:
	}
	return nil
}

func nextNestedContentModel(particles []Particle, next *int) (ContentModelID, bool) {
	for *next < len(particles) && particles[*next].Kind != ParticleModel {
		*next++
	}
	if *next == len(particles) {
		return NoContentModel, false
	}
	child := particles[*next].Model
	*next++
	return child, true
}

// ComplexContentExtendsBase reports whether derived preserves base as the
// leading content of a complex-type extension.
func ComplexContentExtendsBase(rt *schemaBuild, baseID, derivedID ContentModelID) bool {
	if baseID == derivedID || ModelHasNoParticles(rt, baseID) {
		return true
	}
	base, ok := ContentModelByID(rt.Models, baseID)
	if !ok {
		return false
	}
	derived, ok := ContentModelByID(rt.Models, derivedID)
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
		modelParticlePreservesBase(rt.Models, derived.Particles[0], baseID, base)
}

func modelParticlePreservesBase(models []ContentModel, particle Particle, baseID ContentModelID, base ContentModel) bool {
	if base.Occurs.IsExactlyOne() {
		return particle == ModelParticle(baseID, base.Occurs)
	}
	if particle.Kind != ParticleModel || particle.Occurs != base.Occurs || !ValidContentModelID(particle.Model, len(models)) {
		return false
	}
	// CompileModelParticle moves the base occurrence onto the reference and
	// retains its term in an exactly-one clone. Compare that term, not its ID.
	normalized := models[particle.Model]
	return normalized.Occurs.IsExactlyOne() && normalized.Kind == base.Kind &&
		normalized.Mixed == base.Mixed && slices.Equal(normalized.Particles, base.Particles)
}

func spendContentModelWork(work ContentModelWork) error {
	if err := requireContentModelWork(work); err != nil {
		return err
	}
	return work(1)
}

func requireContentModelWork(work ContentModelWork) error {
	if work == nil {
		return errors.New("content-model work budget is required")
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
