package schema

import (
	"errors"
	"slices"
)

// modelKind identifies the runtime content-model shape.
type modelKind uint8

const (
	// modelEmpty is an empty content model.
	modelEmpty modelKind = iota
	// modelAny is the builtin xs:anyType content model.
	modelAny
	// modelSequence is an xs:sequence content model.
	modelSequence
	// modelChoice is an xs:choice content model.
	modelChoice
	// modelAll is an xs:all content model.
	modelAll
)

// particleKind identifies the active reference in a contentParticle.
type particleKind uint8

const (
	// particleElement references an element declaration.
	particleElement particleKind = iota
	// particleModel references a nested content model.
	particleModel
	// particleWildcard references a wildcard.
	particleWildcard
)

// occurrence stores min/max occurrence constraints.
type occurrence struct {
	Min       uint32
	Max       uint32
	Unbounded bool
}

// IsExactlyOne reports whether the occurrence range is exactly 1..1.
func (o occurrence) IsExactlyOne() bool {
	return o.Min == 1 && o.Max == 1 && !o.Unbounded
}

// contentModelNode is a runtime content-model tree node.
type contentModelNode struct {
	Particles []contentParticle
	Occurs    occurrence
	Kind      modelKind
	Mixed     bool
}

// contentModelByID resolves a content model from a content-model table. It
// returns the stored model: admitted models are immutable, the Particles slice
// is shared and has no spare capacity, and callers must not write its elements.
func contentModelByID(models []contentModelNode, id contentModelID) (contentModelNode, bool) {
	if !validContentModelID(id, len(models)) {
		return contentModelNode{}, false
	}
	return models[id], true
}

// contentParticle is a tagged union: Kind selects which ID field is active.
type contentParticle struct {
	Kind     particleKind
	Occurs   occurrence
	Element  ElementID
	Model    contentModelID
	Wildcard WildcardID
}

// contentModelRefLimits are table sizes used to validate cross-table particle
// references in a frozen runtime schema.
type contentModelRefLimits struct {
	ElementCount      int
	ContentModelCount int
	WildcardCount     int
}

// particleRestrictionElement is the element-declaration projection needed to
// evaluate particle restriction over frozen runtime metadata.
type particleRestrictionElement struct {
	Identities IdentityConstraintIDs
	Fixed      constraintIdentity
	Type       TypeID
	Block      DerivationMask
	Scope      declarationScope
	Nillable   bool
}

// elementParticle returns an element particle with inactive fields pinned.
func elementParticle(id ElementID, occurs occurrence) contentParticle {
	return contentParticle{Kind: particleElement, Occurs: occurs, Element: id, Model: noContentModel, Wildcard: NoWildcard}
}

// modelParticle returns a model particle with inactive fields pinned.
func modelParticle(id contentModelID, occurs occurrence) contentParticle {
	return contentParticle{Kind: particleModel, Occurs: occurs, Element: NoElement, Model: id, Wildcard: NoWildcard}
}

// wildcardParticle returns a wildcard particle with inactive fields pinned.
func wildcardParticle(id WildcardID, occurs occurrence) contentParticle {
	return contentParticle{Kind: particleWildcard, Occurs: occurs, Element: NoElement, Model: noContentModel, Wildcard: id}
}

// validateContentModelShape validates content-model metadata that does not
// require cross-table ID lookup.
func validateContentModelShape(model contentModelNode) error {
	if err := validateContentModelKindShape(model); err != nil {
		return err
	}
	for _, particle := range model.Particles {
		if err := validateParticleShape(particle); err != nil {
			return err
		}
	}
	return nil
}

func validateContentModelKindShape(model contentModelNode) error {
	switch model.Kind {
	case modelEmpty:
		return validateEmptyContentModelShape(model)
	case modelAny:
		return validateAnyContentModelShape(model)
	case modelSequence, modelChoice:
		return validateCompositorContentModelShape(model)
	case modelAll:
		return validateAllContentModelShape(model)
	default:
		return errors.New("content model has invalid kind")
	}
}

func validateEmptyContentModelShape(model contentModelNode) error {
	if len(model.Particles) != 0 || model.Occurs != (occurrence{}) {
		return errors.New("empty content model stores inactive fields")
	}
	return nil
}

func validateAnyContentModelShape(model contentModelNode) error {
	if len(model.Particles) != 0 || model.Occurs != (occurrence{}) || !model.Mixed {
		return errors.New("any content model has invalid shape")
	}
	return nil
}

func validateCompositorContentModelShape(model contentModelNode) error {
	if !validOccurrence(model.Occurs) {
		return errors.New("content model occurrence is invalid")
	}
	return nil
}

func validateAllContentModelShape(model contentModelNode) error {
	if !validOccurrence(model.Occurs) || model.Occurs.Unbounded || model.Occurs.Max > 1 || model.Occurs.Min > 1 {
		return errors.New("all content model occurrence is invalid")
	}
	return nil
}

// validateContentModelRuntime validates content-model shape and cross-table
// particle references.
func validateContentModelRuntime(model contentModelNode, limits contentModelRefLimits) error {
	if err := validateContentModelShape(model); err != nil {
		return err
	}
	for _, particle := range model.Particles {
		if err := validateParticleRuntimeReference(particle, limits); err != nil {
			return err
		}
	}
	return nil
}

func validateParticleRuntimeReference(particle contentParticle, limits contentModelRefLimits) error {
	switch particle.Kind {
	case particleElement:
		if !validElementID(particle.Element, limits.ElementCount) {
			return errors.New("particle references invalid element")
		}
	case particleModel:
		if !validContentModelID(particle.Model, limits.ContentModelCount) {
			return errors.New("particle references invalid content model")
		}
	case particleWildcard:
		if !validWildcardID(particle.Wildcard, limits.WildcardCount) {
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
	id   contentModelID
	next int
}

func validateContentModelGraph(models []contentModelNode) error {
	audit := contentModelGraphAudit{
		models: models,
		state:  make([]contentModelGraphState, len(models)),
		stack:  make([]contentModelGraphFrame, 0, min(len(models), 1_024)),
	}
	for i := range models {
		root := contentModelID(i)
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
	models []contentModelNode
	state  []contentModelGraphState
	stack  []contentModelGraphFrame
}

func (a *contentModelGraphAudit) validateRoot(root contentModelID) error {
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
	if !validContentModelID(child, len(a.models)) {
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

func nextNestedContentModel(particles []contentParticle, next *int) (contentModelID, bool) {
	for *next < len(particles) && particles[*next].Kind != particleModel {
		*next++
	}
	if *next == len(particles) {
		return noContentModel, false
	}
	child := particles[*next].Model
	*next++
	return child, true
}

// complexContentExtendsBase reports whether derived preserves base as the
// leading content of a complex-type extension.
func complexContentExtendsBase(rt *schemaBuild, baseID, derivedID contentModelID) bool {
	if baseID == derivedID || modelHasNoParticles(rt, baseID) {
		return true
	}
	base, ok := contentModelByID(rt.Models, baseID)
	if !ok {
		return false
	}
	derived, ok := contentModelByID(rt.Models, derivedID)
	if !ok {
		return false
	}
	if derived.Kind != modelSequence {
		return false
	}
	if !derived.Occurs.IsExactlyOne() {
		return false
	}
	if base.Kind == modelSequence && base.Occurs.IsExactlyOne() {
		return len(derived.Particles) >= len(base.Particles) &&
			slices.Equal(derived.Particles[:len(base.Particles)], base.Particles)
	}
	return len(derived.Particles) != 0 &&
		modelParticlePreservesBase(rt.Models, derived.Particles[0], baseID, base)
}

func modelParticlePreservesBase(models []contentModelNode, particle contentParticle, baseID contentModelID, base contentModelNode) bool {
	if base.Occurs.IsExactlyOne() {
		return particle == modelParticle(baseID, base.Occurs)
	}
	if particle.Kind != particleModel || particle.Occurs != base.Occurs || !validContentModelID(particle.Model, len(models)) {
		return false
	}
	// CompileModelParticle moves the base occurrence onto the reference and
	// retains its term in an exactly-one clone. Compare that term, not its ID.
	normalized := models[particle.Model]
	return normalized.Occurs.IsExactlyOne() && normalized.Kind == base.Kind &&
		normalized.Mixed == base.Mixed && slices.Equal(normalized.Particles, base.Particles)
}

func spendContentModelWork(work contentModelWork) error {
	if err := requireContentModelWork(work); err != nil {
		return err
	}
	return work(1)
}

func requireContentModelWork(work contentModelWork) error {
	if work == nil {
		return errors.New("content-model work budget is required")
	}
	return nil
}

func validOccurrence(o occurrence) bool {
	if o.Unbounded {
		return o.Max == 0
	}
	return o.Max >= o.Min
}

// validateParticleShape validates particle metadata that does not require
// cross-table ID lookup.
func validateParticleShape(p contentParticle) error {
	if !validOccurrence(p.Occurs) {
		return errors.New("particle occurrence is invalid")
	}
	switch p.Kind {
	case particleElement, particleModel, particleWildcard:
	default:
		return errors.New("particle has invalid kind")
	}
	if p.Kind != particleElement && p.Element != NoElement {
		return errors.New("particle stores element ID for non-element kind")
	}
	if p.Kind != particleModel && p.Model != noContentModel {
		return errors.New("particle stores content model ID for non-model kind")
	}
	if p.Kind != particleWildcard && p.Wildcard != NoWildcard {
		return errors.New("particle stores wildcard ID for non-wildcard kind")
	}
	return nil
}
