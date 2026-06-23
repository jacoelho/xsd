package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// ComplexExtensionModelAdmission is the compile-time projection needed to
// validate an extension model before it is appended to its base content.
type ComplexExtensionModelAdmission struct {
	Extension     runtime.ContentModelID
	BaseContent   runtime.ContentModelID
	BaseIsAnyType bool
	BaseMixed     bool
	Mixed         bool
}

// ComplexExtensionContentAdmission is the compile-time projection needed before
// an extension model is lowered.
type ComplexExtensionContentAdmission struct {
	BaseSimpleContent bool
	HasModelChild     bool
}

// ModelChildKind is the compile-time projection of the child term being
// admitted into a model group.
type ModelChildKind uint8

const (
	// ModelChildElement admits an element particle.
	ModelChildElement ModelChildKind = iota
	// ModelChildModel admits a nested model group particle.
	ModelChildModel
	// ModelChildWildcard admits a wildcard particle.
	ModelChildWildcard
)

// ModelChildAdmission is the compile-time projection needed to validate a
// model group child without exposing schema syntax to internal compile code.
type ModelChildAdmission struct {
	Kind      ModelChildKind
	ModelKind runtime.ModelKind
}

// AddContentModelFunc appends a generated content model to the compiler's
// mutable runtime model table.
type AddContentModelFunc func(runtime.ContentModel) (runtime.ContentModelID, error)

// ModelKindForLocal classifies an XSD model-group element local name.
func ModelKindForLocal(local string) (runtime.ModelKind, error) {
	switch local {
	case vocab.XSDElemSequence:
		return runtime.ModelSequence, nil
	case vocab.XSDElemChoice:
		return runtime.ModelChoice, nil
	case vocab.XSDElemAll:
		return runtime.ModelAll, nil
	default:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported model "+local)
	}
}

// ModelChildAdmissionForLocal classifies an XSD model-group child local name.
func ModelChildAdmissionForLocal(local string) (ModelChildAdmission, error) {
	switch local {
	case vocab.XSDElemElement:
		return ModelChildAdmission{Kind: ModelChildElement}, nil
	case vocab.XSDElemAny:
		return ModelChildAdmission{Kind: ModelChildWildcard}, nil
	case vocab.XSDElemGroup:
		return ModelChildAdmission{Kind: ModelChildModel}, nil
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll:
		kind, err := ModelKindForLocal(local)
		if err != nil {
			return ModelChildAdmission{}, err
		}
		return ModelChildAdmissionForModelKind(kind), nil
	default:
		return ModelChildAdmission{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "invalid model group child "+local)
	}
}

// ModelChildAdmissionForModelKind classifies a compiled nested model group.
func ModelChildAdmissionForModelKind(kind runtime.ModelKind) ModelChildAdmission {
	return ModelChildAdmission{Kind: ModelChildModel, ModelKind: kind}
}

// ValidateModelGroupChildAdmission validates compile-time model group child
// admission rules.
func ValidateModelGroupChildAdmission(parent runtime.ModelKind, child ModelChildAdmission) error {
	if parent == runtime.ModelAll && child.Kind != ModelChildElement {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "xs:all can contain only element particles")
	}
	if child.Kind == ModelChildModel && child.ModelKind == runtime.ModelAll {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "xs:all cannot be nested in model groups")
	}
	return nil
}

// AppendParticle applies compile-time particle admission rules and appends p to
// model. Zero-count particles are omitted from the lowered model.
func AppendParticle(model *runtime.ContentModel, p runtime.Particle) error {
	if err := ValidateModelGroupChildAdmission(model.Kind, ModelChildAdmission{Kind: modelParticleChildKind(p)}); err != nil {
		return err
	}
	if p.Occurs.Max == 0 && !p.Occurs.Unbounded {
		return nil
	}
	if model.Kind == runtime.ModelAll && (p.Occurs.Unbounded || p.Occurs.Max > 1) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "xs:all particles cannot repeat")
	}
	model.Particles = append(model.Particles, p)
	return nil
}

func modelParticleChildKind(p runtime.Particle) ModelChildKind {
	switch p.Kind {
	case runtime.ParticleElement:
		return ModelChildElement
	case runtime.ParticleWildcard:
		return ModelChildWildcard
	default:
		return ModelChildModel
	}
}

// ValidateComplexExtensionModelAdmission validates compile-time complex-content
// extension model admission rules.
func ValidateComplexExtensionModelAdmission(rt runtime.ParticleRuntime, admission ComplexExtensionModelAdmission) error {
	if !admission.BaseIsAnyType && admission.BaseMixed && !admission.Mixed {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot drop mixed base content")
	}
	extension, ok := rt.ContentModel(admission.Extension)
	if !ok {
		return xsderrors.InternalInvariant("complex extension references missing extension content model")
	}
	if extension.Kind == runtime.ModelAll && !runtime.ModelHasNoParticles(rt, admission.BaseContent) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot use xs:all")
	}
	if admission.BaseContent == runtime.NoContentModel {
		return nil
	}
	base, ok := rt.ContentModel(admission.BaseContent)
	if !ok {
		return xsderrors.InternalInvariant("complex extension references missing base content model")
	}
	if base.Kind == runtime.ModelAll && len(base.Particles) != 0 {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot add particles to xs:all base")
	}
	return nil
}

// ValidateComplexExtensionContentAdmission validates compile-time complex-content
// extension admission rules that are knowable before model lowering.
func ValidateComplexExtensionContentAdmission(admission ComplexExtensionContentAdmission) error {
	if admission.BaseSimpleContent && admission.HasModelChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot add particles to simple content")
	}
	return nil
}

// ExtendSequenceModel lowers a complex-content extension by combining base and
// extension content into a sequence model when both sides contribute particles.
func ExtendSequenceModel(rt runtime.ParticleRuntime, add AddContentModelFunc, baseID, extID runtime.ContentModelID) (runtime.ContentModelID, error) {
	if baseID == runtime.NoContentModel {
		return extID, nil
	}
	base, ok := rt.ContentModel(baseID)
	if !ok {
		return runtime.NoContentModel, xsderrors.InternalInvariant("sequence extension references missing base content model")
	}
	ext, ok := rt.ContentModel(extID)
	if !ok {
		return runtime.NoContentModel, xsderrors.InternalInvariant("sequence extension references missing extension content model")
	}
	mixed := base.Mixed || ext.Mixed
	if runtime.ModelHasNoParticles(rt, baseID) {
		return ModelWithMixed(rt, add, extID, mixed)
	}
	if runtime.ModelHasNoParticles(rt, extID) {
		return ModelWithMixed(rt, add, baseID, mixed)
	}
	m := runtime.ContentModel{Kind: runtime.ModelSequence, Occurs: runtime.Occurrence{Min: 1, Max: 1}, Mixed: mixed}
	if base.Kind == runtime.ModelSequence && base.Occurs.IsExactlyOne() {
		m.Particles = append(m.Particles, base.Particles...)
	} else if err := AppendModelParticle(rt, add, &m, baseID); err != nil {
		return runtime.NoContentModel, err
	}
	if ext.Kind == runtime.ModelSequence && ext.Occurs.IsExactlyOne() {
		m.Particles = append(m.Particles, ext.Particles...)
	} else if err := AppendModelParticle(rt, add, &m, extID); err != nil {
		return runtime.NoContentModel, err
	}
	return add(m)
}

// ModelWithMixed returns id when its mixed flag already matches, or appends a
// copy with the requested mixed flag.
func ModelWithMixed(rt runtime.ParticleRuntime, add AddContentModelFunc, id runtime.ContentModelID, mixed bool) (runtime.ContentModelID, error) {
	if id == runtime.NoContentModel {
		return id, nil
	}
	model, ok := rt.ContentModel(id)
	if !ok {
		return runtime.NoContentModel, xsderrors.InternalInvariant("model mixed update references missing content model")
	}
	if model.Mixed == mixed {
		return id, nil
	}
	model.Mixed = mixed
	return add(model)
}

// AppendModelParticle appends a model particle for id when the referenced
// model can contribute content.
func AppendModelParticle(rt runtime.ParticleRuntime, add AddContentModelFunc, model *runtime.ContentModel, id runtime.ContentModelID) error {
	p, ok, err := ModelParticle(rt, add, id)
	if err != nil || !ok {
		return err
	}
	model.Particles = append(model.Particles, p)
	return nil
}

// ModelParticle lowers a referenced content model into a particle. Repeated
// model references are normalized through a generated exactly-one model slot.
func ModelParticle(rt runtime.ParticleRuntime, add AddContentModelFunc, id runtime.ContentModelID) (runtime.Particle, bool, error) {
	model, ok := rt.ContentModel(id)
	if !ok {
		return runtime.Particle{}, false, xsderrors.InternalInvariant("model particle references missing content model")
	}
	occurs := model.Occurs
	if occurs.Max == 0 && !occurs.Unbounded {
		return runtime.Particle{}, false, nil
	}
	modelID := id
	if !occurs.IsExactlyOne() {
		normalized := model
		normalized.Occurs = runtime.Occurrence{Min: 1, Max: 1}
		var err error
		modelID, err = add(normalized)
		if err != nil {
			return runtime.Particle{}, false, err
		}
	}
	return runtime.ModelParticle(modelID, occurs), true, nil
}

// AppendFlattenedModelChild appends child particles directly into model when
// the nested model can be lowered without changing occurrence semantics.
func AppendFlattenedModelChild(model *runtime.ContentModel, child runtime.ContentModel) bool {
	if model.Kind == runtime.ModelChoice && child.Kind == runtime.ModelChoice && child.Occurs.IsExactlyOne() {
		model.Particles = append(model.Particles, child.Particles...)
		return true
	}
	if model.Kind != runtime.ModelSequence {
		return false
	}
	if (child.Kind == runtime.ModelSequence || child.Kind == runtime.ModelChoice) && len(child.Particles) == 1 {
		p := child.Particles[0]
		if canFlattenSingleParticleModel(child.Occurs, p.Occurs) {
			p.Occurs = runtime.MultiplyOccurrence(p.Occurs, child.Occurs)
			model.Particles = append(model.Particles, p)
			return true
		}
	}
	if child.Kind == runtime.ModelSequence && len(child.Particles) > 1 && child.Occurs.IsExactlyOne() {
		model.Particles = append(model.Particles, child.Particles...)
		return true
	}
	return false
}

func canFlattenSingleParticleModel(modelOccurs, particleOccurs runtime.Occurrence) bool {
	return modelOccurs.IsExactlyOne() ||
		particleOccurs.Min == 0 ||
		particleOccurs.IsExactlyOne() ||
		(particleOccurs.Unbounded && (modelOccurs.Min > 0 || particleOccurs.Min == 1)) ||
		(!modelOccurs.Unbounded && modelOccurs.Min == modelOccurs.Max)
}
