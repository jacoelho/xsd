package compile

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

type contentRestrictionValidator struct {
	rt runtime.ParticleRestrictionRuntime
}

// ValidateContentRestriction validates content-model restriction.
func ValidateContentRestriction(rt runtime.ParticleRestrictionRuntime, baseID, derivedID runtime.ContentModelID) error {
	if rt == nil {
		return xsderrors.InternalInvariant("content restriction requires runtime")
	}
	validator := contentRestrictionValidator{rt: rt}
	return validator.validateContentRestriction(baseID, derivedID)
}

func (v contentRestrictionValidator) validateContentRestriction(baseID, derivedID runtime.ContentModelID) error {
	if baseID == runtime.NoContentModel || derivedID == runtime.NoContentModel {
		return nil
	}
	base, err := v.contentModel(baseID)
	if err != nil {
		return err
	}
	derived, err := v.contentModel(derivedID)
	if err != nil {
		return err
	}
	if runtime.ModelEmptiable(v, derivedID) && !runtime.ModelEmptiable(v, baseID) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "content restriction is not subset of base")
	}
	if !runtime.OccurrenceRangeSubset(runtime.ModelCountRange(v, derivedID), runtime.ModelCountRange(v, baseID)) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "content restriction is not subset of base")
	}
	if base.Kind == runtime.ModelAny {
		return nil
	}
	if runtime.ModelHasNoParticles(v, derivedID) {
		return nil
	}
	if len(base.Particles) == 1 && base.Particles[0].Kind == runtime.ParticleWildcard {
		for _, p := range derived.Particles {
			if err := v.validateParticleRestrictsWildcard(base.Particles[0], p); err != nil {
				return err
			}
		}
		return nil
	}
	if handled, err := v.validateKnownGroupRestriction(base, derived); handled || err != nil {
		return err
	}
	if v.modelContainsWildcard(derived) && !v.modelContainsWildcard(base) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "wildcard restriction is not subset of base")
	}
	if base.Kind != derived.Kind || len(base.Particles) != len(derived.Particles) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "content restriction is not subset of base")
	}
	for i := range base.Particles {
		if err := v.validateParticleRestriction(base.Particles[i], derived.Particles[i]); err != nil {
			return err
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateKnownGroupRestriction(base, derived runtime.ContentModel) (bool, error) {
	if base.Kind == runtime.ModelChoice && derived.Kind == runtime.ModelChoice {
		return true, v.validateChoiceRestriction(base, derived)
	}
	if base.Kind == runtime.ModelSequence && derived.Kind == runtime.ModelSequence {
		return true, v.validateOrderedGroupRestriction(base, derived, "sequence restriction is not subset of base")
	}
	if base.Kind == runtime.ModelSequence && derived.Kind == runtime.ModelChoice {
		return true, v.validateChoiceRestrictsSequence(base, derived)
	}
	if base.Kind == runtime.ModelAll && derived.Kind == runtime.ModelAll {
		return true, v.validateOrderedGroupRestriction(base, derived, "all restriction is not subset of base")
	}
	if base.Kind == runtime.ModelAll && derived.Kind == runtime.ModelSequence {
		return true, v.validateSequenceRestrictsAll(base, derived)
	}
	if base.Kind == runtime.ModelChoice && derived.Kind == runtime.ModelSequence {
		return true, v.validateSequenceRestrictsChoice(base, derived)
	}
	if base.Kind == runtime.ModelSequence && derived.Kind == runtime.ModelAll {
		if len(base.Particles) == 1 && len(derived.Particles) == 1 {
			return true, v.validateParticleRestriction(base.Particles[0], derived.Particles[0])
		}
		return true, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "all restriction is not subset of sequence")
	}
	return false, nil
}

func (v contentRestrictionValidator) choiceRestrictionBranchAllowed(base []runtime.Particle, derived runtime.Particle) bool {
	for _, b := range base {
		if v.choiceBranchRestricts(b, derived) {
			return true
		}
	}
	return false
}

func (v contentRestrictionValidator) validateChoiceRestriction(base, derived runtime.ContentModel) error {
	if !runtime.OccurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction occurrence is not subset of base")
	}
	if v.choiceRestrictionRequiresXSD11(base, derived) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction requires XSD 1.1 intensional rules")
	}
	baseIndex := 0
	for _, derivedParticle := range derived.Particles {
		matched := false
		for baseIndex < len(base.Particles) {
			if v.choiceBranchRestricts(base.Particles[baseIndex], derivedParticle) {
				baseIndex++
				matched = true
				break
			}
			baseIndex++
		}
		if !matched {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction branch is not subset of base")
		}
	}
	return nil
}

func (v contentRestrictionValidator) choiceRestrictionRequiresXSD11(base, derived runtime.ContentModel) bool {
	if base.Occurs.IsExactlyOne() && derived.Occurs.Min < base.Occurs.Min {
		return true
	}
	if base.Occurs.IsExactlyOne() && derived.Occurs.IsExactlyOne() && len(derived.Particles) < len(base.Particles) {
		for _, p := range derived.Particles {
			if p.Kind == runtime.ParticleModel && runtime.ParticleCountRange(v, p).Unbounded {
				return true
			}
		}
	}
	return slices.ContainsFunc(derived.Particles, v.particleContainsNestedChoice)
}

func (v contentRestrictionValidator) particleContainsNestedChoice(p runtime.Particle) bool {
	if p.Kind != runtime.ParticleModel {
		return false
	}
	model, ok := v.rt.ContentModel(p.Model)
	if !ok {
		return false
	}
	return v.modelContainsChoiceBelow(model, 0)
}

func (v contentRestrictionValidator) modelContainsChoiceBelow(model runtime.ContentModel, depth int) bool {
	if depth > 0 && model.Kind == runtime.ModelChoice {
		return true
	}
	for _, p := range model.Particles {
		if p.Kind != runtime.ParticleModel {
			continue
		}
		nested, ok := v.rt.ContentModel(p.Model)
		if ok && v.modelContainsChoiceBelow(nested, depth+1) {
			return true
		}
	}
	return false
}

func (v contentRestrictionValidator) choiceBranchRestricts(base, derived runtime.Particle) bool {
	candidate := derived
	if base.Kind != runtime.ParticleModel && base.Occurs.IsExactlyOne() && v.particleNeedsChoiceBranchNormalization(derived) {
		candidate.Occurs = runtime.Occurrence{Min: 1, Max: 1}
	}
	return v.validateParticleRestriction(base, candidate) == nil
}

func (v contentRestrictionValidator) particleNeedsChoiceBranchNormalization(p runtime.Particle) bool {
	if runtime.ParticleEffectiveMin(v, p) > 0 {
		return true
	}
	r := runtime.ParticleCountRange(v, p)
	return r.Unbounded || r.Max > 1
}

func (v contentRestrictionValidator) validateOrderedGroupRestriction(base, derived runtime.ContentModel, msg string) error {
	if !runtime.OccurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, msg)
	}
	baseIndex := 0
	for _, derivedParticle := range derived.Particles {
		matched := false
		for baseIndex < len(base.Particles) {
			err := v.validateParticleRestriction(base.Particles[baseIndex], derivedParticle)
			if err == nil {
				baseIndex++
				matched = true
				break
			}
			if xsderrors.IsUnsupported(err) {
				return err
			}
			if !runtime.ParticleEmptiable(v, base.Particles[baseIndex]) {
				return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, msg)
			}
			baseIndex++
		}
		if !matched {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, msg)
		}
	}
	for ; baseIndex < len(base.Particles); baseIndex++ {
		if !runtime.ParticleEmptiable(v, base.Particles[baseIndex]) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, msg)
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateSequenceRestrictsAll(base, derived runtime.ContentModel) error {
	if !runtime.OccurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction occurrence is not subset of all")
	}
	return v.validateMappedGroupRestriction(base, derived, "sequence restriction particle is not subset of all", "sequence restriction omits required all particle")
}

func (v contentRestrictionValidator) validateMappedGroupRestriction(base, derived runtime.ContentModel, particleMsg, omittedMsg string) error {
	mapped := make([]bool, len(base.Particles))
	for _, derivedParticle := range derived.Particles {
		match := -1
		var unsupportedErr error
		for i, baseParticle := range base.Particles {
			if mapped[i] {
				continue
			}
			err := v.validateParticleRestriction(baseParticle, derivedParticle)
			if err == nil {
				match = i
				break
			}
			if xsderrors.IsUnsupported(err) {
				unsupportedErr = err
			}
		}
		if match < 0 {
			if unsupportedErr != nil {
				return unsupportedErr
			}
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, particleMsg)
		}
		mapped[match] = true
	}
	for i, baseParticle := range base.Particles {
		if !mapped[i] && !runtime.ParticleEmptiable(v, baseParticle) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, omittedMsg)
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateSequenceRestrictsChoice(base, derived runtime.ContentModel) error {
	if !runtime.OccurrenceRangeSubset(runtime.SequenceChoiceRange(derived), base.Occurs) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction occurrence is not subset of choice")
	}
	for _, derivedParticle := range derived.Particles {
		if !v.choiceRestrictionBranchAllowed(base.Particles, derivedParticle) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction particle is not subset of choice")
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateChoiceRestrictsSequence(base, derived runtime.ContentModel) error {
	if derived.Occurs.Max == 0 && !derived.Occurs.Unbounded {
		return nil
	}
	if derived.Occurs.Unbounded || derived.Occurs.Max > 1 {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction occurrence is not subset of sequence")
	}
	for _, derivedParticle := range derived.Particles {
		if err := v.validateChoiceBranchRestrictsSequence(base, derivedParticle); err != nil {
			return err
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateChoiceBranchRestrictsSequence(base runtime.ContentModel, derived runtime.Particle) error {
	var unsupportedErr error
	for i, baseParticle := range base.Particles {
		err := v.validateParticleRestriction(baseParticle, derived)
		if err != nil {
			if xsderrors.IsUnsupported(err) {
				unsupportedErr = err
			}
			continue
		}
		if v.sequenceRemainderEmptiable(base.Particles, i) {
			return nil
		}
	}
	if unsupportedErr != nil {
		return unsupportedErr
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction branch is not subset of sequence")
}

func (v contentRestrictionValidator) sequenceRemainderEmptiable(particles []runtime.Particle, selected int) bool {
	for i, p := range particles {
		if i != selected && !runtime.ParticleEmptiable(v, p) {
			return false
		}
	}
	return true
}

func (v contentRestrictionValidator) modelContainsWildcard(model runtime.ContentModel) bool {
	return slices.ContainsFunc(model.Particles, v.particleContainsWildcard)
}

func (v contentRestrictionValidator) particleContainsWildcard(p runtime.Particle) bool {
	switch p.Kind {
	case runtime.ParticleWildcard:
		return true
	case runtime.ParticleModel:
		model, ok := v.rt.ContentModel(p.Model)
		return ok && v.modelContainsWildcard(model)
	default:
		return false
	}
}

func (v contentRestrictionValidator) validateParticleRestriction(base, derived runtime.Particle) error {
	if !runtime.OccurrenceRangeSubset(runtime.ParticleCountRange(v, derived), runtime.ParticleCountRange(v, base)) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "particle restriction occurrence is not subset of base")
	}
	switch base.Kind {
	case runtime.ParticleWildcard:
		return v.validateParticleRestrictsWildcard(base, derived)
	case runtime.ParticleElement:
		return v.validateParticleRestrictsElement(base, derived)
	case runtime.ParticleModel:
		return v.validateParticleRestrictsModel(base, derived)
	default:
		return nil
	}
}

func (v contentRestrictionValidator) validateParticleRestrictsElement(base, derived runtime.Particle) error {
	switch derived.Kind {
	case runtime.ParticleWildcard:
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "wildcard restriction is not subset of element")
	case runtime.ParticleModel:
		model, err := v.contentModel(derived.Model)
		if err != nil {
			return err
		}
		if model.Kind == runtime.ModelChoice {
			for _, p := range model.Particles {
				if !v.choiceBranchRestricts(base, p) {
					return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction branch is not subset of element")
				}
			}
			return nil
		}
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "model group restriction is not subset of element")
	case runtime.ParticleElement:
	default:
		return nil
	}
	baseDecl, err := v.elementRestriction(base.Element)
	if err != nil {
		return err
	}
	derivedDecl, err := v.elementRestriction(derived.Element)
	if err != nil {
		return err
	}
	baseName, err := v.elementName(base.Element)
	if err != nil {
		return err
	}
	derivedName, err := v.elementName(derived.Element)
	if err != nil {
		return err
	}
	if baseName != derivedName {
		allowed, err := v.elementRestrictionNameAllowed(base.Element, derived.Element)
		if err != nil {
			return err
		}
		if !allowed {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction name is not subset of base")
		}
	}
	if mask, ok := runtime.TypeDerivationMask(v.rt, derivedDecl.Type, baseDecl.Type); !ok || mask&runtime.DerivationExtension != 0 {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction type is not derived from base")
	}
	if derivedDecl.Nillable && !baseDecl.Nillable {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction nillable is not subset of base")
	}
	if derivedDecl.Block&baseDecl.Block != baseDecl.Block {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction block is not subset of base")
	}
	if baseDecl.Fixed.Present && !runtime.FixedValueConstraintEqual(baseDecl.Fixed, derivedDecl.Fixed) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction fixed value is not subset of base")
	}
	return nil
}

func (v contentRestrictionValidator) validateParticleRestrictsModel(base, derived runtime.Particle) error {
	model, err := v.contentModel(base.Model)
	if err != nil {
		return err
	}
	if model.Kind == runtime.ModelChoice {
		return v.validateParticleRestrictsChoiceModel(base, derived, model)
	}
	if len(model.Particles) == 1 {
		return v.validateParticleRestriction(model.Particles[0], derived)
	}
	if derived.Kind == runtime.ParticleWildcard {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "wildcard restriction is not subset of model group")
	}
	if model.Kind == runtime.ModelSequence && derived.Kind == runtime.ParticleElement {
		return v.validateElementParticleRestrictsSequenceModel(model, derived)
	}
	if derived.Kind != runtime.ParticleModel {
		return nil
	}
	return v.validateContentRestriction(base.Model, derived.Model)
}

func (v contentRestrictionValidator) validateParticleRestrictsChoiceModel(base, derived runtime.Particle, model runtime.ContentModel) error {
	if derived.Kind == runtime.ParticleModel {
		derivedModel, err := v.contentModel(derived.Model)
		if err != nil {
			return err
		}
		if derivedModel.Kind == runtime.ModelChoice && derived.Occurs.Min < base.Occurs.Min {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction occurrence is not subset of base")
		}
		switch derivedModel.Kind {
		case runtime.ModelChoice:
			return v.validateChoiceRestriction(model, derivedModel)
		case runtime.ModelSequence:
			return v.validateSequenceRestrictsChoice(model, derivedModel)
		default:
		}
	}
	if !v.choiceRestrictionBranchAllowed(model.Particles, derived) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction branch is not subset of base")
	}
	return nil
}

func (v contentRestrictionValidator) validateElementParticleRestrictsSequenceModel(base runtime.ContentModel, derived runtime.Particle) error {
	var unsupportedErr error
	for i, baseParticle := range base.Particles {
		err := v.validateParticleRestriction(baseParticle, derived)
		if err == nil {
			if !v.sequenceRemainderEmptiable(base.Particles, i) {
				return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction omits required base particle")
			}
			return nil
		}
		if unsupportedErr == nil && xsderrors.IsUnsupported(err) {
			unsupportedErr = err
		}
	}
	if unsupportedErr != nil {
		return unsupportedErr
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction particle is not subset of base")
}

func (v contentRestrictionValidator) elementRestrictionNameAllowed(baseID, derivedID runtime.ElementID) (bool, error) {
	derivedName, err := v.elementName(derivedID)
	if err != nil {
		return false, err
	}
	allowed, ok := v.rt.SubstitutionMemberByName(baseID, derivedName)
	return ok && allowed == derivedID, nil
}

func (v contentRestrictionValidator) validateParticleRestrictsWildcard(base, derived runtime.Particle) error {
	switch derived.Kind {
	case runtime.ParticleElement:
		baseWildcard, err := v.wildcard(base.Wildcard)
		if err != nil {
			return err
		}
		derivedName, err := v.elementName(derived.Element)
		if err != nil {
			return err
		}
		if !runtime.WildcardAllowsNamespace(baseWildcard, derivedName.Namespace) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction is not allowed by wildcard")
		}
	case runtime.ParticleWildcard:
		derivedWildcard, err := v.wildcard(derived.Wildcard)
		if err != nil {
			return err
		}
		baseWildcard, err := v.wildcard(base.Wildcard)
		if err != nil {
			return err
		}
		if !runtime.WildcardSubset(derivedWildcard, baseWildcard) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "wildcard restriction is not subset of base")
		}
	case runtime.ParticleModel:
		model, err := v.contentModel(derived.Model)
		if err != nil {
			return err
		}
		for _, child := range model.Particles {
			if err := v.validateParticleRestrictsWildcard(base, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v contentRestrictionValidator) contentModel(id runtime.ContentModelID) (runtime.ContentModel, error) {
	model, ok := v.rt.ContentModel(id)
	if !ok {
		return runtime.ContentModel{}, xsderrors.InternalInvariant("content restriction references missing content model")
	}
	return model, nil
}

func (v contentRestrictionValidator) ContentModel(id runtime.ContentModelID) (runtime.ContentModel, bool) {
	return v.rt.ContentModel(id)
}

func (v contentRestrictionValidator) ElementName(id runtime.ElementID) (runtime.QName, bool) {
	return v.rt.ElementName(id)
}

func (v contentRestrictionValidator) Wildcard(id runtime.WildcardID) (runtime.Wildcard, bool) {
	return v.rt.Wildcard(id)
}

func (v contentRestrictionValidator) ForEachSubstitutionMember(id runtime.ElementID, fn func(runtime.ElementID) bool) {
	v.rt.ForEachSubstitutionMember(id, fn)
}

func (v contentRestrictionValidator) SubstitutionMemberByName(id runtime.ElementID, name runtime.QName) (runtime.ElementID, bool) {
	return v.rt.SubstitutionMemberByName(id, name)
}

func (v contentRestrictionValidator) elementName(id runtime.ElementID) (runtime.QName, error) {
	name, ok := v.rt.ElementName(id)
	if !ok {
		return runtime.QName{}, xsderrors.InternalInvariant("content restriction references missing element name")
	}
	return name, nil
}

func (v contentRestrictionValidator) elementRestriction(id runtime.ElementID) (runtime.ParticleRestrictionElement, error) {
	decl, ok := v.rt.ElementRestriction(id)
	if !ok {
		return runtime.ParticleRestrictionElement{}, xsderrors.InternalInvariant("content restriction references missing element declaration")
	}
	return decl, nil
}

func (v contentRestrictionValidator) wildcard(id runtime.WildcardID) (runtime.Wildcard, error) {
	wildcard, ok := v.Wildcard(id)
	if !ok {
		return runtime.Wildcard{}, xsderrors.InternalInvariant("content restriction references missing wildcard")
	}
	return wildcard, nil
}
