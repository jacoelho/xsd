package schema

import (
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

// modelKind identifies the runtime content-model shape.
type modelKind uint8

const (
	// modelEmpty is an empty content model.
	modelEmpty modelKind = iota
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

// IsAbsent reports whether a particle with this occurrence range corresponds
// to no component. The XML representations of xs:element (Structures
// §3.3.2), xs:group (§3.7.2), xs:sequence and xs:choice (§3.8.2) map an item
// with minOccurs and maxOccurs 0 to no component at all, so the item's own
// references are neither resolved nor import-authorized. Every decision that
// drops a particle source item, or the particle it lowers to, uses it.
func (o occurrence) IsAbsent() bool {
	return o.Max == 0 && !o.Unbounded
}

// IsExactlyOne reports whether the occurrence range is exactly 1..1.
func (o occurrence) IsExactlyOne() bool {
	return o.Min == 1 && o.Max == 1 && !o.Unbounded
}

// Repeats reports whether the occurrence range admits more than one
// occurrence.
func (o occurrence) Repeats() bool {
	return o.Unbounded || o.Max > 1
}

// contentModelNode is a model group of the model table as declared: lowering
// drops absent particles (occurrence.IsAbsent) and keeps every declared group
// as a particle of its own, and executionModel derives what compilation
// executes. It records no mixed flag: the {content type} variety alone says
// whether content is mixed.
type contentModelNode struct {
	Particles []contentParticle
	Occurs    occurrence
	Kind      modelKind
	// NonUnitParticle reports, for a record of the model table, whether the
	// occurrence of some particle is not exactly one. appendModelParticles
	// sets it as it admits each particle, and a copy of a record keeps it with
	// the particles it shares, so the runtime-split check reads it instead of
	// the particles however many records share them. A node built outside
	// the table leaves it unset, so the check reads it only from a record it
	// looks up by contentModelID (modelNeedsRuntimeSplitSeen).
	NonUnitParticle bool
}

// chargedParticles returns the particles of model after charging work one
// step for the model and one for each particle. A record that varies another
// model's occurrence, a root or the unit-occurrence term of a repeated base
// root, shares that model's particles at one dependency step, so the particles
// across the table are not bounded by the dependency steps that admitted them.
// The walk to the models complex types reach (complexTypeReachableModels), the
// walk to the components instances reach (detachUnreachableComponents) and the
// publication table audit (validateContentModelTable) read particles only
// through it; Element
// Declarations Consistent and the per-root analyses charge each particle as
// they visit it.
func chargedParticles(work contentModelWork, model contentModelNode) ([]contentParticle, error) {
	if err := work(1 + len(model.Particles)); err != nil {
		return nil, err
	}
	return model.Particles, nil
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

// contentModelRefLimits are the sizes of the element and wildcard tables that
// content-model particles reference.
type contentModelRefLimits struct {
	ElementCount  int
	WildcardCount int
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
	case modelSequence, modelChoice:
		return validateCompositorContentModelShape(model)
	case modelAll:
		return validateAllContentModelShape(model)
	default:
		return newBuildDefect("content model has invalid kind")
	}
}

func validateEmptyContentModelShape(model contentModelNode) error {
	if len(model.Particles) != 0 || model.Occurs != (occurrence{}) {
		return newBuildDefect("empty content model stores inactive fields")
	}
	return nil
}

func validateCompositorContentModelShape(model contentModelNode) error {
	if !validOccurrence(model.Occurs) {
		return newBuildDefect("content model occurrence is invalid")
	}
	return nil
}

func validateAllContentModelShape(model contentModelNode) error {
	if !validOccurrence(model.Occurs) || model.Occurs.Unbounded || model.Occurs.Max > 1 || model.Occurs.Min > 1 {
		return newBuildDefect("all content model occurrence is invalid")
	}
	return nil
}

func validateParticleRuntimeReference(particle contentParticle, limits contentModelRefLimits, modelCount int) error {
	switch particle.Kind {
	case particleElement:
		if !validElementID(particle.Element, limits.ElementCount) {
			return newBuildDefect("particle references invalid element")
		}
	case particleModel:
		if !validContentModelID(particle.Model, modelCount) {
			return newBuildDefect("particle references invalid content model")
		}
	case particleWildcard:
		if !validWildcardID(particle.Wildcard, limits.WildcardCount) {
			return newBuildDefect("particle references invalid wildcard")
		}
	default:
		return newBuildDefect("particle has invalid kind")
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
	particles []contentParticle
	id        contentModelID
	next      int
}

// validateContentModelTable proves that every model has a valid shape and a
// NonUnitParticle flag that agrees with its particles, that every particle is
// well formed and references a valid element, wildcard or model, and that the
// models form an acyclic graph. It enters every model once and reads its
// particles through chargedParticles.
func validateContentModelTable(models []contentModelNode, limits contentModelRefLimits, work contentModelWork) error {
	if err := requireContentModelWork(work); err != nil {
		return auditError("", err)
	}
	audit := contentModelTableAudit{
		models: models,
		work:   work,
		state:  make([]contentModelGraphState, len(models)),
		stack:  make([]contentModelGraphFrame, 0, min(len(models), 1_024)),
		limits: limits,
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

type contentModelTableAudit struct {
	models []contentModelNode
	work   contentModelWork
	state  []contentModelGraphState
	stack  []contentModelGraphFrame
	limits contentModelRefLimits
}

func (a *contentModelTableAudit) enter(id contentModelID) error {
	model := a.models[id]
	particles, err := chargedParticles(a.work, model)
	if err != nil {
		return err
	}
	if err := validateContentModelKindShape(model); err != nil {
		return auditError("", err)
	}
	if model.NonUnitParticle != slices.ContainsFunc(particles, nonUnitParticle) {
		return xsderrors.InternalInvariant("content model non-unit particle flag disagrees with its particles", nil)
	}
	a.state[id] = contentModelGraphChecking
	a.stack = appendDFSFrame(a.stack, contentModelGraphFrame{particles: particles, id: id}, len(a.models))
	return nil
}

func (a *contentModelTableAudit) validateRoot(root contentModelID) error {
	if err := a.enter(root); err != nil {
		return err
	}
	for len(a.stack) != 0 {
		if err := a.advance(); err != nil {
			return err
		}
	}
	return nil
}

func (a *contentModelTableAudit) advance() error {
	top := len(a.stack) - 1
	frame := &a.stack[top]
	if frame.next == len(frame.particles) {
		a.state[frame.id] = contentModelGraphChecked
		a.stack = a.stack[:top]
		return nil
	}
	particle := frame.particles[frame.next]
	frame.next++
	if err := validateParticleShape(particle); err != nil {
		return auditError("", err)
	}
	if err := validateParticleRuntimeReference(particle, a.limits, len(a.models)); err != nil {
		return auditError("", err)
	}
	if particle.Kind != particleModel {
		return nil
	}
	switch a.state[particle.Model] {
	case contentModelGraphUnchecked:
		return a.enter(particle.Model)
	case contentModelGraphChecking:
		return xsderrors.InternalInvariant("content model graph contains cycle", nil)
	case contentModelGraphChecked:
	}
	return nil
}

// complexContentExtendsBase reports whether derived's {content type}, its
// variety and particle, is the one Structures §3.4.2 complex content clause
// 3.2 gives an extension of base: any content over an empty base (3.2.2), and
// otherwise the base variety (Derivation Valid (Extension) clause 1.4.3.2.2.1)
// with the base particle itself (3.2.1) or a 1..1 sequence whose first
// particle is the base particle (3.2.3).
func complexContentExtendsBase(rt *schemaBuild, base, derived complexTypeMetadata) bool {
	if base.ContentKind == contentEmpty {
		return true
	}
	if derived.ContentKind != base.ContentKind {
		return false
	}
	if derived.Content == base.Content {
		return true
	}
	model, ok := contentModelByID(rt.Models, derived.Content)
	if !ok || model.Kind != modelSequence || !model.Occurs.IsExactlyOne() || len(model.Particles) != 2 {
		return false
	}
	first := model.Particles[0]
	baseModel, ok := contentModelByID(rt.Models, base.Content)
	if !ok {
		return false
	}
	if baseModel.Kind == modelEmpty {
		return first.Kind == particleModel && first.Occurs.IsExactlyOne() &&
			validContentModelID(first.Model, len(rt.Models)) && emptySequence(rt.Models[first.Model])
	}
	return modelParticlePreservesBase(rt.Models, first, base.Content, baseModel)
}

func emptySequence(model contentModelNode) bool {
	return model.Kind == modelSequence && model.Occurs.IsExactlyOne() && len(model.Particles) == 0
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
		slices.Equal(normalized.Particles, base.Particles)
}

func spendContentModelWork(work contentModelWork) error {
	if err := requireContentModelWork(work); err != nil {
		return err
	}
	return work(1)
}

func requireContentModelWork(work contentModelWork) error {
	if work == nil {
		return newBuildDefect("content-model work budget is required")
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
		return newBuildDefect("particle occurrence is invalid")
	}
	switch p.Kind {
	case particleElement, particleModel, particleWildcard:
	default:
		return newBuildDefect("particle has invalid kind")
	}
	if p.Kind != particleElement && p.Element != NoElement {
		return newBuildDefect("particle stores element ID for non-element kind")
	}
	if p.Kind != particleModel && p.Model != noContentModel {
		return newBuildDefect("particle stores content model ID for non-model kind")
	}
	if p.Kind != particleWildcard && p.Wildcard != NoWildcard {
		return newBuildDefect("particle stores wildcard ID for non-wildcard kind")
	}
	return nil
}
