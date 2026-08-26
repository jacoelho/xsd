package runtime

import (
	"errors"
)

type contentRestrictionErrorKind uint8

const (
	contentRestrictionMismatchKind contentRestrictionErrorKind = iota
	contentRestrictionInvariantKind
)

type contentRestrictionError struct {
	message string
	kind    contentRestrictionErrorKind
}

func (e *contentRestrictionError) Error() string { return e.message }

func contentRestrictionMismatch(message string) error {
	return &contentRestrictionError{message: message, kind: contentRestrictionMismatchKind}
}

func contentRestrictionInvariant(message string) error {
	return &contentRestrictionError{message: message, kind: contentRestrictionInvariantKind}
}

// IsContentRestrictionMismatch reports a valid restriction relation that is
// not a subset of its base.
func IsContentRestrictionMismatch(err error) bool {
	issue, ok := errors.AsType[*contentRestrictionError](err)
	return ok && issue.kind == contentRestrictionMismatchKind
}

// IsContentRestrictionInvariant reports invalid runtime metadata encountered
// while evaluating a restriction relation.
func IsContentRestrictionInvariant(err error) bool {
	issue, ok := errors.AsType[*contentRestrictionError](err)
	return ok && issue.kind == contentRestrictionInvariantKind
}

type contentRestrictionValidator struct {
	rt          ParticleRestrictionRuntime
	analysis    *ContentModelAnalysis
	modelStates map[ContentModelID]uint8
	wildcards   map[ContentModelID]bool
	choiceBelow map[ContentModelID]bool
	work        *contentModelWorkState
}

// ContentModelWork charges bounded content-model analysis work.
type ContentModelWork func(steps int) error

type contentModelWorkState struct {
	spend ContentModelWork
	err   error
}

// ValidateContentRestriction validates content-model restriction while
// charging every graph traversal and particle comparison to work.
func ValidateContentRestriction(
	rt ParticleRestrictionRuntime,
	baseID, derivedID ContentModelID,
	work ContentModelWork,
	analysis *ContentModelAnalysis,
) error {
	if rt == nil {
		return contentRestrictionInvariant("content restriction requires runtime")
	}
	if err := requireContentModelWork(work); err != nil {
		return err
	}
	if analysis == nil {
		return contentRestrictionInvariant("content restriction requires content model analysis")
	}
	validator := newContentRestrictionValidator(rt, work, analysis)
	err := validator.validateContentRestriction(baseID, derivedID)
	return validator.finish(err)
}

func newContentRestrictionValidator(
	rt ParticleRestrictionRuntime,
	work ContentModelWork,
	analysis *ContentModelAnalysis,
) contentRestrictionValidator {
	return contentRestrictionValidator{
		rt:          rt,
		analysis:    analysis,
		modelStates: make(map[ContentModelID]uint8),
		wildcards:   make(map[ContentModelID]bool),
		choiceBelow: make(map[ContentModelID]bool),
		work:        &contentModelWorkState{spend: work},
	}
}

func (v contentRestrictionValidator) charge() error {
	return v.spend(1)
}

func (v contentRestrictionValidator) spend(steps int) error {
	if v.work.err == nil {
		v.work.err = v.work.spend(steps)
	}
	return v.work.err
}

func (v contentRestrictionValidator) finish(err error) error {
	if v.work.err != nil {
		return v.work.err
	}
	return err
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
	derivedEmptiable, err := v.analysis.ModelEmptiable(derivedID)
	if err != nil {
		return err
	}
	baseEmptiable, err := v.analysis.ModelEmptiable(baseID)
	if err != nil {
		return err
	}
	if derivedEmptiable && !baseEmptiable {
		return contentRestrictionMismatch("content restriction is not subset of base")
	}
	derivedRange, err := v.analysis.ModelCountRange(derivedID)
	if err != nil {
		return err
	}
	baseRange, err := v.analysis.ModelCountRange(baseID)
	if err != nil {
		return err
	}
	if !OccurrenceRangeSubset(derivedRange, baseRange) {
		return contentRestrictionMismatch("content restriction is not subset of base")
	}
	if base.Kind == ModelAny {
		return nil
	}
	hasNoParticles, err := v.modelHasNoParticles(derivedID)
	if err != nil {
		return err
	}
	if hasNoParticles {
		return nil
	}
	if len(base.Particles) == 1 && base.Particles[0].Kind == ParticleWildcard {
		for _, p := range derived.Particles {
			if wildcardErr := v.validateParticleRestrictsWildcard(base.Particles[0], p); wildcardErr != nil {
				return wildcardErr
			}
		}
		return nil
	}
	if handled, groupErr := v.validateKnownGroupRestriction(base, derived); handled || groupErr != nil {
		return groupErr
	}
	derivedContainsWildcard, err := v.modelContainsWildcard(derived)
	if err != nil {
		return err
	}
	baseContainsWildcard, err := v.modelContainsWildcard(base)
	if err != nil {
		return err
	}
	if derivedContainsWildcard && !baseContainsWildcard {
		return contentRestrictionMismatch("wildcard restriction is not subset of base")
	}
	if base.Kind != derived.Kind || len(base.Particles) != len(derived.Particles) {
		return contentRestrictionMismatch("content restriction is not subset of base")
	}
	for i := range base.Particles {
		if err := v.validateParticleRestriction(base.Particles[i], derived.Particles[i]); err != nil {
			return err
		}
	}
	return nil
}

func (v contentRestrictionValidator) modelHasNoParticles(id ContentModelID) (bool, error) {
	if id == NoContentModel {
		return true, nil
	}
	model, err := v.contentModel(id)
	if err != nil {
		return false, err
	}
	switch model.Kind {
	case ModelEmpty:
		return true, nil
	case ModelSequence, ModelChoice, ModelAll:
		return len(model.Particles) == 0, nil
	default:
		return false, nil
	}
}

func (v contentRestrictionValidator) particleEffectiveMin(particle Particle) (uint32, error) {
	if particle.Kind == ParticleModel {
		emptiable, err := v.analysis.ModelEmptiable(particle.Model)
		if err != nil {
			return 0, err
		}
		if emptiable {
			return 0, nil
		}
	}
	return particle.Occurs.Min, nil
}

func (v contentRestrictionValidator) validateContentModelGraph(id ContentModelID) error {
	states := v.modelStates
	if states == nil {
		states = make(map[ContentModelID]uint8)
	}
	return v.validateContentModelGraphWithStates(id, states)
}

func (v contentRestrictionValidator) validateContentModelGraphWithStates(id ContentModelID, states map[ContentModelID]uint8) error {
	if err := v.charge(); err != nil {
		return err
	}
	const (
		modelChecking uint8 = iota + 1
		modelChecked
	)
	switch states[id] {
	case modelChecking:
		return contentRestrictionInvariant("content restriction references cyclic content model")
	case modelChecked:
		return nil
	}
	model, err := v.contentModel(id)
	if err != nil {
		return err
	}
	if shapeErr := ValidateContentModelShape(model); shapeErr != nil {
		return contentRestrictionInvariant("content restriction references invalid content model: " + shapeErr.Error())
	}
	states[id] = modelChecking
	for _, particle := range model.Particles {
		if err := v.charge(); err != nil {
			return err
		}
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
				return contentRestrictionInvariant("content restriction references element declaration with invalid scope")
			}
		case ParticleWildcard:
			if _, err := v.wildcard(particle.Wildcard); err != nil {
				return err
			}
		default:
			return contentRestrictionInvariant("content restriction references invalid particle kind")
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
		return true, contentRestrictionMismatch("all restriction is not subset of sequence")
	}
	return false, nil
}

func (v contentRestrictionValidator) choiceRestrictionBranchAllowed(base []Particle, derived Particle) (bool, error) {
	for _, b := range base {
		allowed, err := v.choiceBranchRestricts(b, derived)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

func (v contentRestrictionValidator) validateChoiceRestriction(base, derived ContentModel) error {
	if !OccurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return contentRestrictionMismatch("choice restriction occurrence is not subset of base")
	}
	requiresXSD11, err := v.choiceRestrictionRequiresXSD11(base, derived)
	if err != nil {
		return err
	}
	if requiresXSD11 {
		return contentRestrictionMismatch("choice restriction requires XSD 1.1 intensional rules")
	}
	baseIndex := 0
	for _, derivedParticle := range derived.Particles {
		matched := false
		for baseIndex < len(base.Particles) {
			allowed, err := v.choiceBranchRestricts(base.Particles[baseIndex], derivedParticle)
			if err != nil {
				return err
			}
			if allowed {
				matched = true
				break
			}
			baseIndex++
		}
		if !matched {
			return contentRestrictionMismatch("choice restriction branch is not subset of base")
		}
	}
	return nil
}

func (v contentRestrictionValidator) choiceRestrictionRequiresXSD11(base, derived ContentModel) (bool, error) {
	if base.Occurs.IsExactlyOne() && derived.Occurs.Min < base.Occurs.Min {
		return true, nil
	}
	if base.Occurs.IsExactlyOne() && derived.Occurs.IsExactlyOne() && len(derived.Particles) < len(base.Particles) {
		for _, p := range derived.Particles {
			if p.Kind != ParticleModel {
				continue
			}
			rangeForParticle, err := v.analysis.ParticleCountRange(p)
			if err != nil {
				return false, err
			}
			if rangeForParticle.Unbounded {
				return true, nil
			}
		}
	}
	for _, particle := range derived.Particles {
		contains, err := v.particleContainsNestedChoice(particle)
		if err != nil {
			return false, err
		}
		if contains {
			return true, nil
		}
	}
	return false, nil
}

func (v contentRestrictionValidator) particleContainsNestedChoice(p Particle) (bool, error) {
	if err := v.charge(); err != nil {
		return false, err
	}
	if p.Kind != ParticleModel {
		return false, nil
	}
	return v.modelContainsChoiceBelow(p.Model)
}

func (v contentRestrictionValidator) modelContainsChoiceBelow(id ContentModelID) (bool, error) {
	if result, ok := v.choiceBelow[id]; ok {
		return result, nil
	}
	model, err := v.contentModel(id)
	if err != nil {
		return false, err
	}
	for _, p := range model.Particles {
		if err := v.charge(); err != nil {
			return false, err
		}
		if p.Kind != ParticleModel {
			continue
		}
		nested, err := v.contentModel(p.Model)
		if err != nil {
			return false, err
		}
		if nested.Kind == ModelChoice {
			v.choiceBelow[id] = true
			return true, nil
		}
		contains, err := v.modelContainsChoiceBelow(p.Model)
		if err != nil {
			return false, err
		}
		if contains {
			v.choiceBelow[id] = true
			return true, nil
		}
	}
	v.choiceBelow[id] = false
	return false, nil
}

func (v contentRestrictionValidator) choiceBranchRestricts(base, derived Particle) (bool, error) {
	candidate := derived
	if base.Kind != ParticleModel && base.Occurs.IsExactlyOne() {
		normalize, err := v.particleNeedsChoiceBranchNormalization(derived)
		if err != nil {
			return false, err
		}
		if normalize {
			candidate.Occurs = Occurrence{Min: 1, Max: 1}
		}
	}
	err := v.validateParticleRestriction(base, candidate)
	if err == nil {
		return true, nil
	}
	if IsContentRestrictionMismatch(err) {
		return false, nil
	}
	return false, err
}

func (v contentRestrictionValidator) particleNeedsChoiceBranchNormalization(p Particle) (bool, error) {
	effectiveMin, err := v.particleEffectiveMin(p)
	if err != nil {
		return false, err
	}
	if effectiveMin > 0 {
		return true, nil
	}
	rangeForParticle, err := v.analysis.ParticleCountRange(p)
	if err != nil {
		return false, err
	}
	return rangeForParticle.Unbounded || rangeForParticle.Max > 1, nil
}

func (v contentRestrictionValidator) validateOrderedGroupRestriction(base, derived ContentModel, msg string) error {
	if !OccurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return contentRestrictionMismatch(msg)
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
			if !IsContentRestrictionMismatch(err) {
				return err
			}
			emptiable, emptiableErr := v.analysis.ParticleEmptiable(base.Particles[baseIndex])
			if emptiableErr != nil {
				return emptiableErr
			}
			if !emptiable {
				return contentRestrictionMismatch(msg)
			}
			baseIndex++
		}
		if !matched {
			return contentRestrictionMismatch(msg)
		}
	}
	for ; baseIndex < len(base.Particles); baseIndex++ {
		emptiable, err := v.analysis.ParticleEmptiable(base.Particles[baseIndex])
		if err != nil {
			return err
		}
		if !emptiable {
			return contentRestrictionMismatch(msg)
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateSequenceRestrictsAll(base, derived ContentModel) error {
	if !OccurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return contentRestrictionMismatch("sequence restriction occurrence is not subset of all")
	}
	return v.validateMappedGroupRestriction(base, derived, "sequence restriction particle is not subset of all", "sequence restriction omits required all particle")
}

func (v contentRestrictionValidator) validateMappedGroupRestriction(base, derived ContentModel, particleMsg, omittedMsg string) error {
	mapped := make([]bool, len(base.Particles))
	for _, derivedParticle := range derived.Particles {
		match := -1
		for i, baseParticle := range base.Particles {
			if mapped[i] {
				continue
			}
			err := v.validateParticleRestriction(baseParticle, derivedParticle)
			if err == nil {
				match = i
				break
			}
			if !IsContentRestrictionMismatch(err) {
				return err
			}
		}
		if match < 0 {
			return contentRestrictionMismatch(particleMsg)
		}
		mapped[match] = true
	}
	for i, baseParticle := range base.Particles {
		if mapped[i] {
			continue
		}
		emptiable, err := v.analysis.ParticleEmptiable(baseParticle)
		if err != nil {
			return err
		}
		if !emptiable {
			return contentRestrictionMismatch(omittedMsg)
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateSequenceRestrictsChoice(base, derived ContentModel) error {
	if !OccurrenceRangeSubset(SequenceChoiceRange(derived), base.Occurs) {
		return contentRestrictionMismatch("sequence restriction occurrence is not subset of choice")
	}
	for _, derivedParticle := range derived.Particles {
		allowed, err := v.choiceRestrictionBranchAllowed(base.Particles, derivedParticle)
		if err != nil {
			return err
		}
		if !allowed {
			return contentRestrictionMismatch("sequence restriction particle is not subset of choice")
		}
	}
	return nil
}

func (v contentRestrictionValidator) validatePointlessChoiceRestrictsSequence(base, derived ContentModel) error {
	if !derived.Occurs.IsExactlyOne() || len(derived.Particles) != 1 {
		return contentRestrictionMismatch("choice restriction of sequence is forbidden")
	}
	return v.validateChoiceBranchRestrictsSequence(base, derived.Particles[0])
}

func (v contentRestrictionValidator) validateChoiceBranchRestrictsSequence(base ContentModel, derived Particle) error {
	for i, baseParticle := range base.Particles {
		err := v.validateParticleRestriction(baseParticle, derived)
		if err != nil {
			if !IsContentRestrictionMismatch(err) {
				return err
			}
			continue
		}
		emptiable, err := v.sequenceRemainderEmptiable(base.Particles, i)
		if err != nil {
			return err
		}
		if emptiable {
			return nil
		}
	}
	return contentRestrictionMismatch("choice restriction branch is not subset of sequence")
}

func (v contentRestrictionValidator) sequenceRemainderEmptiable(particles []Particle, selected int) (bool, error) {
	for i, p := range particles {
		if i == selected {
			continue
		}
		emptiable, err := v.analysis.ParticleEmptiable(p)
		if err != nil {
			return false, err
		}
		if !emptiable {
			return false, nil
		}
	}
	return true, nil
}

func (v contentRestrictionValidator) modelContainsWildcard(model ContentModel) (bool, error) {
	for _, particle := range model.Particles {
		contains, err := v.particleContainsWildcard(particle)
		if err != nil {
			return false, err
		}
		if contains {
			return true, nil
		}
	}
	return false, nil
}

func (v contentRestrictionValidator) particleContainsWildcard(p Particle) (bool, error) {
	if err := v.charge(); err != nil {
		return false, err
	}
	switch p.Kind {
	case ParticleWildcard:
		return true, nil
	case ParticleModel:
		return v.modelIDContainsWildcard(p.Model)
	default:
		return false, nil
	}
}

func (v contentRestrictionValidator) modelIDContainsWildcard(id ContentModelID) (bool, error) {
	if result, ok := v.wildcards[id]; ok {
		return result, nil
	}
	model, err := v.contentModel(id)
	if err != nil {
		return false, err
	}
	result, err := v.modelContainsWildcard(model)
	if err != nil {
		return false, err
	}
	v.wildcards[id] = result
	return result, nil
}

func (v contentRestrictionValidator) validateParticleRestriction(base, derived Particle) error {
	if err := v.charge(); err != nil {
		return err
	}
	derivedRange, err := v.analysis.ParticleCountRange(derived)
	if err != nil {
		return err
	}
	baseRange, err := v.analysis.ParticleCountRange(base)
	if err != nil {
		return err
	}
	if !OccurrenceRangeSubset(derivedRange, baseRange) {
		return contentRestrictionMismatch("particle restriction occurrence is not subset of base")
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
		return contentRestrictionMismatch("wildcard restriction is not subset of element")
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
					return contentRestrictionMismatch("choice restriction branch is not subset of element")
				}
			}
			return nil
		}
		return contentRestrictionMismatch("model group restriction is not subset of element")
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
		member, ok := v.SubstitutionMemberByName(base.Element, derivedName)
		if !ok || member != derived.Element {
			return contentRestrictionMismatch("element restriction name is not subset of base")
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
		return contentRestrictionInvariant("content restriction references element declaration with invalid scope")
	}
	if baseDecl.Scope == DeclarationScopeGlobal && derivedDecl.Scope == DeclarationScopeGlobal {
		return nil
	}
	const excluded = DerivationExtension | DerivationList | DerivationUnion
	mask, ok, err := typeDerivationMask(v.rt, derivedDecl.Type, baseDecl.Type, v.spend)
	if err != nil {
		return err
	}
	if !ok || mask&excluded != 0 {
		return contentRestrictionMismatch("element restriction type is not derived from base")
	}
	if derivedDecl.Nillable && !baseDecl.Nillable {
		return contentRestrictionMismatch("element restriction nillable is not subset of base")
	}
	if derivedDecl.Block&baseDecl.Block != baseDecl.Block {
		return contentRestrictionMismatch("element restriction block is not subset of base")
	}
	if !derivedDecl.Identities.IsSubsetOf(baseDecl.Identities) {
		return contentRestrictionMismatch("element restriction identity constraints are not subset of base")
	}
	if baseDecl.Fixed.Present && !FixedValueConstraintEqual(baseDecl.Fixed, derivedDecl.Fixed) {
		return contentRestrictionMismatch("element restriction fixed value is not subset of base")
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
		return contentRestrictionMismatch("wildcard restriction is not subset of model group")
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
			return contentRestrictionMismatch("choice restriction occurrence is not subset of base")
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
		return contentRestrictionMismatch("choice restriction branch is not subset of base")
	}
	return nil
}

func (v contentRestrictionValidator) validateElementParticleRestrictsSequenceModel(base ContentModel, derived Particle) error {
	for i, baseParticle := range base.Particles {
		err := v.validateParticleRestriction(baseParticle, derived)
		if err == nil {
			emptiable, emptiableErr := v.sequenceRemainderEmptiable(base.Particles, i)
			if emptiableErr != nil {
				return emptiableErr
			}
			if !emptiable {
				return contentRestrictionMismatch("sequence restriction omits required base particle")
			}
			return nil
		}
		if !IsContentRestrictionMismatch(err) {
			return err
		}
	}
	return contentRestrictionMismatch("sequence restriction particle is not subset of base")
}

func (v contentRestrictionValidator) validateParticleRestrictsWildcard(base, derived Particle) error {
	if err := v.charge(); err != nil {
		return err
	}
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
			return contentRestrictionMismatch("element restriction is not allowed by wildcard")
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
			return contentRestrictionMismatch("wildcard restriction is not subset of base")
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
	model, ok := v.ContentModel(id)
	if !ok {
		return ContentModel{}, contentRestrictionInvariant("content restriction references missing content model")
	}
	return model, nil
}

func (v contentRestrictionValidator) ContentModel(id ContentModelID) (ContentModel, bool) {
	if v.charge() != nil {
		return ContentModel{}, false
	}
	return v.rt.ContentModel(id)
}

func (v contentRestrictionValidator) ElementName(id ElementID) (QName, bool) {
	if v.charge() != nil {
		return QName{}, false
	}
	return v.rt.ElementName(id)
}

func (v contentRestrictionValidator) Wildcard(id WildcardID) (Wildcard, bool) {
	if v.charge() != nil {
		return Wildcard{}, false
	}
	return v.rt.Wildcard(id)
}

func (v contentRestrictionValidator) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	v.rt.ForEachSubstitutionMember(id, func(member ElementID) bool {
		return v.charge() == nil && fn(member)
	})
}

func (v contentRestrictionValidator) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	if v.charge() != nil {
		return NoElement, false
	}
	return v.rt.SubstitutionMemberByName(id, name)
}

func (v contentRestrictionValidator) elementName(id ElementID) (QName, error) {
	name, ok := v.ElementName(id)
	if !ok {
		if err := v.finish(nil); err != nil {
			return QName{}, err
		}
		return QName{}, contentRestrictionInvariant("content restriction references missing element name")
	}
	return name, nil
}

func (v contentRestrictionValidator) elementRestriction(id ElementID) (ParticleRestrictionElement, error) {
	if err := v.charge(); err != nil {
		return ParticleRestrictionElement{}, err
	}
	decl, ok := v.rt.ElementRestriction(id)
	if !ok {
		return ParticleRestrictionElement{}, contentRestrictionInvariant("content restriction references missing element declaration")
	}
	return decl, nil
}

func (v contentRestrictionValidator) wildcard(id WildcardID) (Wildcard, error) {
	wildcard, ok := v.Wildcard(id)
	if !ok {
		return Wildcard{}, contentRestrictionInvariant("content restriction references missing wildcard")
	}
	return wildcard, nil
}
