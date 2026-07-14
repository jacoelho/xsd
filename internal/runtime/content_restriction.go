package runtime

import (
	"errors"
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

type contentRestrictionValidator struct {
	rt          ParticleRestrictionRuntime
	modelStates map[ContentModelID]uint8
}

// ValidateContentRestriction validates content-model restriction.
func ValidateContentRestriction(rt ParticleRestrictionRuntime, baseID, derivedID ContentModelID) error {
	if rt == nil {
		return xsderrors.InternalInvariant("content restriction requires runtime")
	}
	validator := contentRestrictionValidator{rt: rt, modelStates: make(map[ContentModelID]uint8)}
	return validator.validateContentRestriction(baseID, derivedID)
}

func (v contentRestrictionValidator) validateContentRestriction(baseID, derivedID ContentModelID) error {
	if baseID == NoContentModel || derivedID == NoContentModel {
		return nil
	}
	if err := v.validateContentModelGraph(baseID); err != nil {
		return err
	}
	if err := v.validateContentModelGraph(derivedID); err != nil {
		return err
	}
	base, err := v.contentModel(baseID)
	if err != nil {
		return err
	}
	derived, err := v.contentModel(derivedID)
	if err != nil {
		return err
	}
	if ModelEmptiable(v, derivedID) && !ModelEmptiable(v, baseID) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "content restriction is not subset of base")
	}
	if !OccurrenceRangeSubset(ModelCountRange(v, derivedID), ModelCountRange(v, baseID)) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "content restriction is not subset of base")
	}
	if base.Kind == ModelAny {
		return nil
	}
	if ModelHasNoParticles(v, derivedID) {
		return nil
	}
	if len(base.Particles) == 1 && base.Particles[0].Kind == ParticleWildcard {
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

func (v contentRestrictionValidator) validateContentModelGraph(id ContentModelID) error {
	states := v.modelStates
	if states == nil {
		states = make(map[ContentModelID]uint8)
	}
	return v.validateContentModelGraphWithStates(id, states)
}

func (v contentRestrictionValidator) validateContentModelGraphWithStates(id ContentModelID, states map[ContentModelID]uint8) error {
	const (
		modelChecking uint8 = iota + 1
		modelChecked
	)
	switch states[id] {
	case modelChecking:
		return xsderrors.InternalInvariant("content restriction references cyclic content model")
	case modelChecked:
		return nil
	}
	model, err := v.contentModel(id)
	if err != nil {
		return err
	}
	if shapeErr := ValidateContentModelShape(model); shapeErr != nil {
		return xsderrors.InternalInvariant("content restriction references invalid content model: " + shapeErr.Error())
	}
	states[id] = modelChecking
	for _, particle := range model.Particles {
		switch particle.Kind {
		case ParticleModel:
			if err := v.validateContentModelGraphWithStates(particle.Model, states); err != nil {
				return err
			}
		case ParticleElement:
			if _, err := v.elementName(particle.Element); err != nil {
				return err
			}
			decl, err := v.elementRestriction(particle.Element)
			if err != nil {
				return err
			}
			if decl.Scope == DeclarationScopeInvalid {
				return xsderrors.InternalInvariant("content restriction references element declaration with invalid scope")
			}
		case ParticleWildcard:
			if _, err := v.wildcard(particle.Wildcard); err != nil {
				return err
			}
		default:
			return xsderrors.InternalInvariant("content restriction references invalid particle kind")
		}
	}
	states[id] = modelChecked
	return nil
}

func (v contentRestrictionValidator) validateKnownGroupRestriction(base, derived ContentModel) (bool, error) {
	if base.Kind == ModelChoice && derived.Kind == ModelChoice {
		return true, v.validateChoiceRestriction(base, derived)
	}
	if base.Kind == ModelSequence && derived.Kind == ModelSequence {
		return true, v.validateOrderedGroupRestriction(base, derived, "sequence restriction is not subset of base")
	}
	if base.Kind == ModelSequence && derived.Kind == ModelChoice {
		return true, v.validatePointlessChoiceRestrictsSequence(base, derived)
	}
	if base.Kind == ModelAll && derived.Kind == ModelAll {
		return true, v.validateOrderedGroupRestriction(base, derived, "all restriction is not subset of base")
	}
	if base.Kind == ModelAll && derived.Kind == ModelSequence {
		return true, v.validateSequenceRestrictsAll(base, derived)
	}
	if base.Kind == ModelChoice && derived.Kind == ModelSequence {
		return true, v.validateSequenceRestrictsChoice(base, derived)
	}
	if base.Kind == ModelSequence && derived.Kind == ModelAll {
		if len(base.Particles) == 1 && len(derived.Particles) == 1 {
			return true, v.validateParticleRestriction(base.Particles[0], derived.Particles[0])
		}
		return true, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "all restriction is not subset of sequence")
	}
	return false, nil
}

func (v contentRestrictionValidator) choiceRestrictionBranchAllowed(base []Particle, derived Particle) (bool, error) {
	var unsupportedErr error
	for _, b := range base {
		allowed, err := v.choiceBranchRestricts(b, derived)
		if err != nil {
			if xsderrors.IsUnsupported(err) {
				unsupportedErr = err
				continue
			}
			return false, err
		}
		if allowed {
			return true, nil
		}
	}
	return false, unsupportedErr
}

func (v contentRestrictionValidator) validateChoiceRestriction(base, derived ContentModel) error {
	if !OccurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction occurrence is not subset of base")
	}
	if v.choiceRestrictionRequiresXSD11(base, derived) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction requires XSD 1.1 intensional rules")
	}
	baseIndex := 0
	for _, derivedParticle := range derived.Particles {
		matched := false
		var unsupportedErr error
		for baseIndex < len(base.Particles) {
			allowed, err := v.choiceBranchRestricts(base.Particles[baseIndex], derivedParticle)
			if err != nil {
				if xsderrors.IsUnsupported(err) {
					unsupportedErr = err
					baseIndex++
					continue
				}
				return err
			}
			if allowed {
				matched = true
				break
			}
			baseIndex++
		}
		if !matched {
			if unsupportedErr != nil {
				return unsupportedErr
			}
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction branch is not subset of base")
		}
	}
	return nil
}

func (v contentRestrictionValidator) choiceRestrictionRequiresXSD11(base, derived ContentModel) bool {
	if base.Occurs.IsExactlyOne() && derived.Occurs.Min < base.Occurs.Min {
		return true
	}
	if base.Occurs.IsExactlyOne() && derived.Occurs.IsExactlyOne() && len(derived.Particles) < len(base.Particles) {
		for _, p := range derived.Particles {
			if p.Kind == ParticleModel && ParticleCountRange(v, p).Unbounded {
				return true
			}
		}
	}
	return slices.ContainsFunc(derived.Particles, v.particleContainsNestedChoice)
}

func (v contentRestrictionValidator) particleContainsNestedChoice(p Particle) bool {
	if p.Kind != ParticleModel {
		return false
	}
	model, ok := v.rt.ContentModel(p.Model)
	if !ok {
		return false
	}
	return v.modelContainsChoiceBelow(model, 0)
}

func (v contentRestrictionValidator) modelContainsChoiceBelow(model ContentModel, depth int) bool {
	if depth > 0 && model.Kind == ModelChoice {
		return true
	}
	for _, p := range model.Particles {
		if p.Kind != ParticleModel {
			continue
		}
		nested, ok := v.rt.ContentModel(p.Model)
		if ok && v.modelContainsChoiceBelow(nested, depth+1) {
			return true
		}
	}
	return false
}

func (v contentRestrictionValidator) choiceBranchRestricts(base, derived Particle) (bool, error) {
	candidate := derived
	if base.Kind != ParticleModel && base.Occurs.IsExactlyOne() && v.particleNeedsChoiceBranchNormalization(derived) {
		candidate.Occurs = Occurrence{Min: 1, Max: 1}
	}
	err := v.validateParticleRestriction(base, candidate)
	if err == nil {
		return true, nil
	}
	if isContentRestrictionMismatch(err) {
		return false, nil
	}
	return false, err
}

func isContentRestrictionMismatch(err error) bool {
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	return ok && diagnostic.Category == xsderrors.CategorySchemaCompile && diagnostic.Code == xsderrors.CodeSchemaContentModel
}

func (v contentRestrictionValidator) particleNeedsChoiceBranchNormalization(p Particle) bool {
	if ParticleEffectiveMin(v, p) > 0 {
		return true
	}
	r := ParticleCountRange(v, p)
	return r.Unbounded || r.Max > 1
}

func (v contentRestrictionValidator) validateOrderedGroupRestriction(base, derived ContentModel, msg string) error {
	if !OccurrenceRangeSubset(derived.Occurs, base.Occurs) {
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
			if !isContentRestrictionMismatch(err) {
				return err
			}
			if !ParticleEmptiable(v, base.Particles[baseIndex]) {
				return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, msg)
			}
			baseIndex++
		}
		if !matched {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, msg)
		}
	}
	for ; baseIndex < len(base.Particles); baseIndex++ {
		if !ParticleEmptiable(v, base.Particles[baseIndex]) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, msg)
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateSequenceRestrictsAll(base, derived ContentModel) error {
	if !OccurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction occurrence is not subset of all")
	}
	return v.validateMappedGroupRestriction(base, derived, "sequence restriction particle is not subset of all", "sequence restriction omits required all particle")
}

func (v contentRestrictionValidator) validateMappedGroupRestriction(base, derived ContentModel, particleMsg, omittedMsg string) error {
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
			} else if !isContentRestrictionMismatch(err) {
				return err
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
		if !mapped[i] && !ParticleEmptiable(v, baseParticle) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, omittedMsg)
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateSequenceRestrictsChoice(base, derived ContentModel) error {
	if !OccurrenceRangeSubset(SequenceChoiceRange(derived), base.Occurs) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction occurrence is not subset of choice")
	}
	for _, derivedParticle := range derived.Particles {
		allowed, err := v.choiceRestrictionBranchAllowed(base.Particles, derivedParticle)
		if err != nil {
			return err
		}
		if !allowed {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction particle is not subset of choice")
		}
	}
	return nil
}

func (v contentRestrictionValidator) validatePointlessChoiceRestrictsSequence(base, derived ContentModel) error {
	if !derived.Occurs.IsExactlyOne() || len(derived.Particles) != 1 {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction of sequence is forbidden")
	}
	return v.validateChoiceBranchRestrictsSequence(base, derived.Particles[0])
}

func (v contentRestrictionValidator) validateChoiceBranchRestrictsSequence(base ContentModel, derived Particle) error {
	var unsupportedErr error
	for i, baseParticle := range base.Particles {
		err := v.validateParticleRestriction(baseParticle, derived)
		if err != nil {
			if xsderrors.IsUnsupported(err) {
				unsupportedErr = err
			} else if !isContentRestrictionMismatch(err) {
				return err
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

func (v contentRestrictionValidator) sequenceRemainderEmptiable(particles []Particle, selected int) bool {
	for i, p := range particles {
		if i != selected && !ParticleEmptiable(v, p) {
			return false
		}
	}
	return true
}

func (v contentRestrictionValidator) modelContainsWildcard(model ContentModel) bool {
	return slices.ContainsFunc(model.Particles, v.particleContainsWildcard)
}

func (v contentRestrictionValidator) particleContainsWildcard(p Particle) bool {
	switch p.Kind {
	case ParticleWildcard:
		return true
	case ParticleModel:
		model, ok := v.rt.ContentModel(p.Model)
		return ok && v.modelContainsWildcard(model)
	default:
		return false
	}
}

func (v contentRestrictionValidator) validateParticleRestriction(base, derived Particle) error {
	if !OccurrenceRangeSubset(ParticleCountRange(v, derived), ParticleCountRange(v, base)) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "particle restriction occurrence is not subset of base")
	}
	switch base.Kind {
	case ParticleWildcard:
		return v.validateParticleRestrictsWildcard(base, derived)
	case ParticleElement:
		return v.validateParticleRestrictsElement(base, derived)
	case ParticleModel:
		return v.validateParticleRestrictsModel(base, derived)
	default:
		return nil
	}
}

func (v contentRestrictionValidator) validateParticleRestrictsElement(base, derived Particle) error {
	switch derived.Kind {
	case ParticleWildcard:
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "wildcard restriction is not subset of element")
	case ParticleModel:
		model, err := v.contentModel(derived.Model)
		if err != nil {
			return err
		}
		if model.Kind == ModelChoice {
			for _, p := range model.Particles {
				allowed, branchErr := v.choiceBranchRestricts(base, p)
				if branchErr != nil {
					return branchErr
				}
				if !allowed {
					return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction branch is not subset of element")
				}
			}
			return nil
		}
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "model group restriction is not subset of element")
	case ParticleElement:
	default:
		return nil
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
		member, ok := v.rt.SubstitutionMemberByName(base.Element, derivedName)
		if !ok || member != derived.Element {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction name is not subset of base")
		}
		base.Element = member
	}
	baseDecl, err := v.elementRestriction(base.Element)
	if err != nil {
		return err
	}
	derivedDecl, err := v.elementRestriction(derived.Element)
	if err != nil {
		return err
	}
	if baseDecl.Scope == DeclarationScopeInvalid || derivedDecl.Scope == DeclarationScopeInvalid {
		return xsderrors.InternalInvariant("content restriction references element declaration with invalid scope")
	}
	if baseDecl.Scope == DeclarationScopeGlobal && derivedDecl.Scope == DeclarationScopeGlobal {
		return nil
	}
	const excluded = DerivationExtension | DerivationList | DerivationUnion
	if mask, ok := TypeDerivationMask(v.rt, derivedDecl.Type, baseDecl.Type); !ok || mask&excluded != 0 {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction type is not derived from base")
	}
	if derivedDecl.Nillable && !baseDecl.Nillable {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction nillable is not subset of base")
	}
	if derivedDecl.Block&baseDecl.Block != baseDecl.Block {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction block is not subset of base")
	}
	if !derivedDecl.Identities.IsSubsetOf(baseDecl.Identities) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction identity constraints are not subset of base")
	}
	if baseDecl.Fixed.Present && !FixedValueConstraintEqual(baseDecl.Fixed, derivedDecl.Fixed) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction fixed value is not subset of base")
	}
	return nil
}

func (v contentRestrictionValidator) validateParticleRestrictsModel(base, derived Particle) error {
	model, err := v.contentModel(base.Model)
	if err != nil {
		return err
	}
	if model.Kind == ModelChoice {
		return v.validateParticleRestrictsChoiceModel(base, derived, model)
	}
	if len(model.Particles) == 1 {
		return v.validateParticleRestriction(model.Particles[0], derived)
	}
	if derived.Kind == ParticleWildcard {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "wildcard restriction is not subset of model group")
	}
	if model.Kind == ModelSequence && derived.Kind == ParticleElement {
		return v.validateElementParticleRestrictsSequenceModel(model, derived)
	}
	if derived.Kind != ParticleModel {
		return nil
	}
	return v.validateContentRestriction(base.Model, derived.Model)
}

func (v contentRestrictionValidator) validateParticleRestrictsChoiceModel(base, derived Particle, model ContentModel) error {
	if derived.Kind == ParticleModel {
		derivedModel, err := v.contentModel(derived.Model)
		if err != nil {
			return err
		}
		if derivedModel.Kind == ModelChoice && derived.Occurs.Min < base.Occurs.Min {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction occurrence is not subset of base")
		}
		switch derivedModel.Kind {
		case ModelChoice:
			return v.validateChoiceRestriction(model, derivedModel)
		case ModelSequence:
			return v.validateSequenceRestrictsChoice(model, derivedModel)
		default:
		}
	}
	allowed, err := v.choiceRestrictionBranchAllowed(model.Particles, derived)
	if err != nil {
		return err
	}
	if !allowed {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "choice restriction branch is not subset of base")
	}
	return nil
}

func (v contentRestrictionValidator) validateElementParticleRestrictsSequenceModel(base ContentModel, derived Particle) error {
	var unsupportedErr error
	for i, baseParticle := range base.Particles {
		err := v.validateParticleRestriction(baseParticle, derived)
		if err == nil {
			if !v.sequenceRemainderEmptiable(base.Particles, i) {
				return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction omits required base particle")
			}
			return nil
		}
		if xsderrors.IsUnsupported(err) {
			if unsupportedErr == nil {
				unsupportedErr = err
			}
			continue
		}
		if !isContentRestrictionMismatch(err) {
			return err
		}
	}
	if unsupportedErr != nil {
		return unsupportedErr
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "sequence restriction particle is not subset of base")
}

func (v contentRestrictionValidator) validateParticleRestrictsWildcard(base, derived Particle) error {
	switch derived.Kind {
	case ParticleElement:
		baseWildcard, err := v.wildcard(base.Wildcard)
		if err != nil {
			return err
		}
		derivedName, err := v.elementName(derived.Element)
		if err != nil {
			return err
		}
		if !WildcardAllowsNamespace(baseWildcard, derivedName.Namespace) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element restriction is not allowed by wildcard")
		}
	case ParticleWildcard:
		derivedWildcard, err := v.wildcard(derived.Wildcard)
		if err != nil {
			return err
		}
		baseWildcard, err := v.wildcard(base.Wildcard)
		if err != nil {
			return err
		}
		if !WildcardSubset(derivedWildcard, baseWildcard) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "wildcard restriction is not subset of base")
		}
	case ParticleModel:
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

func (v contentRestrictionValidator) contentModel(id ContentModelID) (ContentModel, error) {
	model, ok := v.rt.ContentModel(id)
	if !ok {
		return ContentModel{}, xsderrors.InternalInvariant("content restriction references missing content model")
	}
	return model, nil
}

func (v contentRestrictionValidator) ContentModel(id ContentModelID) (ContentModel, bool) {
	return v.rt.ContentModel(id)
}

func (v contentRestrictionValidator) ElementName(id ElementID) (QName, bool) {
	return v.rt.ElementName(id)
}

func (v contentRestrictionValidator) Wildcard(id WildcardID) (Wildcard, bool) {
	return v.rt.Wildcard(id)
}

func (v contentRestrictionValidator) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	v.rt.ForEachSubstitutionMember(id, fn)
}

func (v contentRestrictionValidator) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	return v.rt.SubstitutionMemberByName(id, name)
}

func (v contentRestrictionValidator) elementName(id ElementID) (QName, error) {
	name, ok := v.rt.ElementName(id)
	if !ok {
		return QName{}, xsderrors.InternalInvariant("content restriction references missing element name")
	}
	return name, nil
}

func (v contentRestrictionValidator) elementRestriction(id ElementID) (ParticleRestrictionElement, error) {
	decl, ok := v.rt.ElementRestriction(id)
	if !ok {
		return ParticleRestrictionElement{}, xsderrors.InternalInvariant("content restriction references missing element declaration")
	}
	return decl, nil
}

func (v contentRestrictionValidator) wildcard(id WildcardID) (Wildcard, error) {
	wildcard, ok := v.Wildcard(id)
	if !ok {
		return Wildcard{}, xsderrors.InternalInvariant("content restriction references missing wildcard")
	}
	return wildcard, nil
}
