package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

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

// appendModelParticles is the only append to the particles of a model the
// model table retains. It charges one MaxSchemaDependencySteps step per
// particle before it allocates, so no model under construction retains memory
// that the dependency budget has not paid for. The restriction normalizer
// builds transient comparison nodes, which it charges to
// MaxContentModelAnalysisSteps and never retains.
func appendModelParticles(work *workBudget, model *contentModelNode, particles ...contentParticle) error {
	if err := work.spend(len(particles)); err != nil {
		return err
	}
	model.Particles = append(model.Particles, particles...)
	return nil
}

// appendParticle admits p as a child of model and appends it. Zero-count
// particles are omitted from the lowered model. The occurrence range of an
// xs:all member was admitted at its start tag; see occurrenceAdmission.
func appendParticle(work *workBudget, model *contentModelNode, p contentParticle) error {
	if err := validateModelGroupChildAdmission(model.Kind, modelChildAdmission{Kind: modelParticleChildKind(p)}); err != nil {
		return err
	}
	if p.Occurs.IsAbsent() {
		return nil
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

// contentTypeParticle returns the particle of the {content type} whose model
// is id, as Structures §3.4.2 complex content clause 3.2.3 composes it. Mixed
// empty content is stored as an empty model with the mixed flag, and
// noContentModel is the effective content of mixed empty explicit content;
// both stand for the synthetic 1..1 empty sequence of clause 2.1.4.
func contentTypeParticle(rt *schemaBuild, add addContentModelFunc, id contentModelID) (contentParticle, error) {
	one := occurrence{Min: 1, Max: 1}
	if id == noContentModel || rt.Models[id].Kind == modelEmpty {
		sequence, err := add(contentModelNode{Kind: modelSequence, Occurs: one})
		return modelParticle(sequence, one), err
	}
	p, ok, err := compileModelParticle(rt, add, id)
	if err != nil {
		return contentParticle{}, err
	}
	if !ok {
		return contentParticle{}, xsderrors.InternalInvariant("content type particle has no occurrence")
	}
	return p, nil
}

// validateExtensionSequenceTerms rejects an all group in the sequence clause
// 3.2.3 builds from the base content model and the effective content: All
// Group Limited admits an all group only as the whole {content type} particle.
// Effective content with no model is mixed empty content, whose particle is
// the synthetic empty sequence of clause 2.1.4.
func validateExtensionSequenceTerms(rt *schemaBuild, base contentModelID, effective effectiveContent) error {
	if effective.model != noContentModel && rt.Models[effective.model].Kind == modelAll {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot use xs:all")
	}
	if rt.Models[base].Kind != modelAll {
		return nil
	}
	if effective.model == noContentModel {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "mixed complexContent extension with empty content cannot extend an xs:all base: its content type is a sequence of the xs:all group and an empty sequence")
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot add particles to xs:all base")
}

// compileModelParticle lowers the root model id of a {content type} into a
// particle. A root carries its occurrence on its record, so a repeated root
// moves its occurrence onto the particle and names a generated exactly-one
// model that shares its particles.
func compileModelParticle(rt *schemaBuild, add addContentModelFunc, id contentModelID) (contentParticle, bool, error) {
	model, ok := contentModelByID(rt.Models, id)
	if !ok {
		return contentParticle{}, false, xsderrors.InternalInvariant("model particle references missing content model")
	}
	occurs := model.Occurs
	if occurs.IsAbsent() {
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
