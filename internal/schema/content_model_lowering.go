package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// complexExtensionModelAdmission is the compile-time projection needed to
// validate an extension model before it is appended to its base content.
type complexExtensionModelAdmission struct {
	Extension     contentModelID
	BaseContent   contentModelID
	BaseIsAnyType bool
	BaseMixed     bool
	Mixed         bool
}

// complexExtensionContentAdmission is the compile-time projection needed before
// an extension model is lowered.
type complexExtensionContentAdmission struct {
	BaseSimpleContent bool
	HasModelChild     bool
}

// modelChildKind is the compile-time projection of the child term being
// admitted into a model group.
type modelChildKind uint8

const (
	// modelChildElement admits an element particle.
	modelChildElement modelChildKind = iota
	// modelChildModel admits a nested model group particle.
	modelChildModel
	// modelChildWildcard admits a wildcard particle.
	modelChildWildcard
)

// modelChildAdmission is the compile-time projection needed to validate a
// model group child without exposing schema syntax to internal compile code.
type modelChildAdmission struct {
	Kind      modelChildKind
	ModelKind modelKind
}

// addContentModelFunc appends a generated content model to the compiler's
// mutable runtime model table.
type addContentModelFunc func(contentModelNode) (contentModelID, error)

type modelTextKind uint8

const (
	modelTextInvalid modelTextKind = iota
	modelTextElementOnly
	modelTextMixed
)

// modelKindForLocal classifies an XSD model-group element local name.
func modelKindForLocal(local string) (modelKind, error) {
	switch local {
	case vocab.XSDElemSequence:
		return modelSequence, nil
	case vocab.XSDElemChoice:
		return modelChoice, nil
	case vocab.XSDElemAll:
		return modelAll, nil
	default:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported model "+diagnostictext.Quote(local))
	}
}

// modelChildAdmissionForModelKind classifies a compiled nested model group.
func modelChildAdmissionForModelKind(kind modelKind) modelChildAdmission {
	return modelChildAdmission{Kind: modelChildModel, ModelKind: kind}
}

// validateModelGroupChildAdmission validates compile-time model group child
// admission rules.
func validateModelGroupChildAdmission(parent modelKind, child modelChildAdmission) error {
	if parent == modelAll && child.Kind != modelChildElement {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "xs:all can contain only element particles")
	}
	if child.Kind == modelChildModel && child.ModelKind == modelAll {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "xs:all cannot be nested in model groups")
	}
	return nil
}

// appendModelParticles is the only production statement that grows
// contentModelNode.Particles. It charges one MaxSchemaDependencySteps step per
// particle before it allocates, so no model under construction retains memory
// that the dependency budget has not paid for.
func appendModelParticles(work *workBudget, model *contentModelNode, particles ...contentParticle) error {
	if err := work.spend(len(particles)); err != nil {
		return err
	}
	model.Particles = append(model.Particles, particles...)
	return nil
}

// appendParticle applies compile-time particle admission rules and appends p to
// model. Zero-count particles are omitted from the lowered model.
func appendParticle(work *workBudget, model *contentModelNode, p contentParticle) error {
	if err := validateModelGroupChildAdmission(model.Kind, modelChildAdmission{Kind: modelParticleChildKind(p)}); err != nil {
		return err
	}
	if p.Occurs.Max == 0 && !p.Occurs.Unbounded {
		return nil
	}
	if model.Kind == modelAll && (p.Occurs.Unbounded || p.Occurs.Max > 1) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "xs:all particles cannot repeat")
	}
	return appendModelParticles(work, model, p)
}

func modelParticleChildKind(p contentParticle) modelChildKind {
	switch p.Kind {
	case particleElement:
		return modelChildElement
	case particleWildcard:
		return modelChildWildcard
	case particleModel:
		return modelChildModel
	default:
	}
	return modelChildModel
}

// validateComplexExtensionModelAdmission validates compile-time complex-content
// extension model admission rules.
func validateComplexExtensionModelAdmission(rt *schemaBuild, admission complexExtensionModelAdmission) error {
	if !admission.BaseIsAnyType && admission.BaseMixed && !admission.Mixed {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot drop mixed base content")
	}
	extension, ok := contentModelByID(rt.Models, admission.Extension)
	if !ok {
		return xsderrors.InternalInvariant("complex extension references missing extension content model")
	}
	if extension.Kind == modelAll && !modelHasNoParticles(rt, admission.BaseContent) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot use xs:all")
	}
	if admission.BaseContent == noContentModel {
		return nil
	}
	base, ok := contentModelByID(rt.Models, admission.BaseContent)
	if !ok {
		return xsderrors.InternalInvariant("complex extension references missing base content model")
	}
	if base.Kind == modelAll && len(base.Particles) != 0 {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot add particles to xs:all base")
	}
	return nil
}

// validateComplexExtensionContentAdmission validates compile-time complex-content
// extension admission rules that are knowable before model lowering.
func validateComplexExtensionContentAdmission(admission complexExtensionContentAdmission) error {
	if admission.BaseSimpleContent && admission.HasModelChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot add particles to simple content")
	}
	return nil
}

// extendSequenceModel lowers a complex-content extension by combining base and
// extension content into a sequence model when both sides contribute particles.
func extendSequenceModel(rt *schemaBuild, add addContentModelFunc, baseID, extID contentModelID) (contentModelID, error) {
	if baseID == noContentModel {
		return extID, nil
	}
	base, ok := contentModelByID(rt.Models, baseID)
	if !ok {
		return noContentModel, xsderrors.InternalInvariant("sequence extension references missing base content model")
	}
	ext, ok := contentModelByID(rt.Models, extID)
	if !ok {
		return noContentModel, xsderrors.InternalInvariant("sequence extension references missing extension content model")
	}
	textKind := modelTextElementOnly
	if base.Mixed || ext.Mixed {
		textKind = modelTextMixed
	}
	if modelHasNoParticles(rt, baseID) {
		return modelWithTextKind(rt, add, extID, textKind)
	}
	if modelHasNoParticles(rt, extID) {
		return modelWithTextKind(rt, add, baseID, textKind)
	}
	m := contentModelNode{Kind: modelSequence, Occurs: occurrence{Min: 1, Max: 1}, Mixed: textKind == modelTextMixed}
	if err := appendSequenceExtensionOperand(rt, add, &m, baseID, base); err != nil {
		return noContentModel, err
	}
	if err := appendSequenceExtensionOperand(rt, add, &m, extID, ext); err != nil {
		return noContentModel, err
	}
	return add(m)
}

func appendSequenceExtensionOperand(rt *schemaBuild, add addContentModelFunc, target *contentModelNode, id contentModelID, model contentModelNode) error {
	if model.Kind == modelSequence && model.Occurs.IsExactlyOne() {
		return appendModelParticles(rt.typeWork, target, model.Particles...)
	}
	return appendModelParticle(rt, add, target, id)
}

func modelWithTextKind(rt *schemaBuild, add addContentModelFunc, id contentModelID, textKind modelTextKind) (contentModelID, error) {
	var mixed bool
	switch textKind {
	case modelTextElementOnly:
	case modelTextMixed:
		mixed = true
	case modelTextInvalid:
		return noContentModel, xsderrors.InternalInvariant("invalid model text kind")
	default:
		return noContentModel, xsderrors.InternalInvariant("unknown model text kind")
	}
	if id == noContentModel {
		return id, nil
	}
	model, ok := contentModelByID(rt.Models, id)
	if !ok {
		return noContentModel, xsderrors.InternalInvariant("model mixed update references missing content model")
	}
	if model.Mixed == mixed {
		return id, nil
	}
	model.Mixed = mixed
	return add(model)
}

// appendModelParticle appends a model particle for id when the referenced
// model can contribute content.
func appendModelParticle(rt *schemaBuild, add addContentModelFunc, model *contentModelNode, id contentModelID) error {
	p, ok, err := compileModelParticle(rt, add, id)
	if err != nil || !ok {
		return err
	}
	return appendModelParticles(rt.typeWork, model, p)
}

// compileModelParticle lowers a referenced content model into a particle. Repeated
// model references are normalized through a generated exactly-one model slot.
func compileModelParticle(rt *schemaBuild, add addContentModelFunc, id contentModelID) (contentParticle, bool, error) {
	model, ok := contentModelByID(rt.Models, id)
	if !ok {
		return contentParticle{}, false, xsderrors.InternalInvariant("model particle references missing content model")
	}
	occurs := model.Occurs
	if occurs.Max == 0 && !occurs.Unbounded {
		return contentParticle{}, false, nil
	}
	modelID := id
	if !occurs.IsExactlyOne() {
		normalized := model
		normalized.Occurs = occurrence{Min: 1, Max: 1}
		var err error
		modelID, err = add(normalized)
		if err != nil {
			return contentParticle{}, false, err
		}
	}
	return modelParticle(modelID, occurs), true, nil
}

// appendFlattenedModelChild removes pointless groups only. Language-equivalent
// occurrence multiplication can erase a structural restriction boundary.
func appendFlattenedModelChild(work *workBudget, model *contentModelNode, child contentModelNode) (bool, error) {
	if model.Kind == modelChoice && child.Kind == modelChoice && child.Occurs.IsExactlyOne() {
		return true, appendModelParticles(work, model, child.Particles...)
	}
	if model.Kind != modelSequence {
		return false, nil
	}
	if (child.Kind == modelSequence || child.Kind == modelChoice) && len(child.Particles) == 1 && child.Occurs.IsExactlyOne() {
		return true, appendModelParticles(work, model, child.Particles[0])
	}
	if child.Kind == modelSequence && len(child.Particles) > 1 && child.Occurs.IsExactlyOne() {
		return true, appendModelParticles(work, model, child.Particles...)
	}
	return false, nil
}
