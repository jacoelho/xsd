package schema

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

// isContentRestrictionMismatch reports a valid restriction relation that is
// not a subset of its base.
func isContentRestrictionMismatch(err error) bool {
	issue, ok := errors.AsType[*contentRestrictionError](err)
	return ok && issue.kind == contentRestrictionMismatchKind
}

// isContentRestrictionInvariant reports invalid runtime metadata encountered
// while evaluating a restriction relation.
func isContentRestrictionInvariant(err error) bool {
	issue, ok := errors.AsType[*contentRestrictionError](err)
	return ok && issue.kind == contentRestrictionInvariantKind
}

type contentRestrictionValidator struct {
	rt          *schemaBuild
	analysis    *contentModelAnalysis
	modelStates map[contentModelID]uint8
	wildcards   map[contentModelID]bool
	work        *contentModelWorkState
}

// contentModelWork charges bounded content-model analysis work.
type contentModelWork func(steps int) error

type contentModelWorkState struct {
	spend contentModelWork
	err   error
}

// validateContentRestriction validates content-model restriction while
// charging every graph traversal and particle comparison to work.
//
// Compilation and publication share this validator. Both operands have their
// actual substitution groups expanded and prescribed pointless groups removed
// before structural dispatch. Group-to-wildcard restriction checks the
// aggregate against the original wildcard and each member against a copy with
// minimum zero (I-004 in docs/spec/INTERPRETATIONS.md). RecurseLax waives
// order only inside generated substitution choices or their flattened spans
// (I-005); explicit siblings keep their order. Matching is bounded and
// functional, with no permutation search. Comparison never mutates build
// models, execution limits or reusable analysis caches, and a failure discards
// the temporary comparison view. Restriction never rewrites the derived
// particle; runtime cardinality comes from the admitted occurrence ranges.
func validateContentRestriction(
	rt *schemaBuild,
	baseID, derivedID contentModelID,
	work contentModelWork,
	analysis *contentModelAnalysis,
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
	rt *schemaBuild,
	work contentModelWork,
	analysis *contentModelAnalysis,
) contentRestrictionValidator {
	return contentRestrictionValidator{
		rt:          rt,
		analysis:    analysis,
		modelStates: make(map[contentModelID]uint8),
		wildcards:   make(map[contentModelID]bool),
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

func (v contentRestrictionValidator) validateContentRestriction(baseID, derivedID contentModelID) error {
	if baseID == noContentModel || derivedID == noContentModel {
		return nil
	}
	_, derived, err := v.restrictionModels(baseID, derivedID)
	if err != nil {
		return err
	}
	if err := v.validateContentRestrictionRanges(baseID, derivedID); err != nil {
		return err
	}
	// Empty content restricts an originally emptiable base (clause 5.3).
	// Pointless-group removal can discard the branch proving emptiability.
	if derived.Kind == modelEmpty || baseID == derivedID {
		return nil
	}
	one := occurrence{Min: 1, Max: 1}
	return v.compareNormalizedRestriction(modelParticle(baseID, one), modelParticle(derivedID, one))
}

func (v contentRestrictionValidator) restrictionModels(
	baseID, derivedID contentModelID,
) (base, derived contentModelNode, err error) {
	if validationErr := v.validateContentModelGraph(baseID); validationErr != nil {
		return contentModelNode{}, contentModelNode{}, validationErr
	}
	if validationErr := v.validateContentModelGraph(derivedID); validationErr != nil {
		return contentModelNode{}, contentModelNode{}, validationErr
	}
	base, err = v.requireContentModel(baseID)
	if err != nil {
		return contentModelNode{}, contentModelNode{}, err
	}
	derived, err = v.requireContentModel(derivedID)
	if err != nil {
		return contentModelNode{}, contentModelNode{}, err
	}
	return base, derived, nil
}

func (v contentRestrictionValidator) validateContentRestrictionRanges(baseID, derivedID contentModelID) error {
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
	if !occurrenceRangeSubset(derivedRange, baseRange) {
		return contentRestrictionMismatch("content restriction is not subset of base")
	}
	return nil
}

func (v contentRestrictionValidator) validateFallbackContentRestriction(base, derived contentModelNode) error {
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
	return contentRestrictionMismatch("content restriction is not subset of base")
}

func (v contentRestrictionValidator) validateContentModelGraph(id contentModelID) error {
	states := v.modelStates
	if states == nil {
		states = make(map[contentModelID]uint8)
	}
	return v.validateContentModelGraphWithStates(id, states)
}

func (v contentRestrictionValidator) validateContentModelGraphWithStates(id contentModelID, states map[contentModelID]uint8) error {
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
	model, err := v.requireContentModel(id)
	if err != nil {
		return err
	}
	if shapeErr := validateContentModelShape(model); shapeErr != nil {
		return contentRestrictionInvariant("content restriction references invalid content model: " + shapeErr.Error())
	}
	states[id] = modelChecking
	for _, particle := range model.Particles {
		if err := v.charge(); err != nil {
			return err
		}
		if err := v.validateContentRestrictionGraphParticle(particle, states); err != nil {
			return err
		}
	}
	states[id] = modelChecked
	return nil
}

func (v contentRestrictionValidator) validateContentRestrictionGraphParticle(
	particle contentParticle,
	states map[contentModelID]uint8,
) error {
	switch particle.Kind {
	case particleModel:
		return v.validateContentModelGraphWithStates(particle.Model, states)
	case particleElement:
		return v.validateContentRestrictionGraphElement(particle.Element)
	case particleWildcard:
		_, err := v.requireWildcard(particle.Wildcard)
		return err
	default:
		return contentRestrictionInvariant("content restriction references invalid particle kind")
	}
}

func (v contentRestrictionValidator) validateContentRestrictionGraphElement(id ElementID) error {
	if _, err := v.requireElementName(id); err != nil {
		return err
	}
	declaration, err := v.elementRestriction(id)
	if err != nil {
		return err
	}
	if declaration.Scope == declarationScopeInvalid {
		return contentRestrictionInvariant("content restriction references element declaration with invalid scope")
	}
	return nil
}

func (v contentRestrictionValidator) validateKnownGroupRestriction(base, derived contentModelNode, baseOrder, derivedOrder restrictionChoiceOrder) (bool, error) {
	switch {
	case base.Kind == modelChoice && derived.Kind == modelChoice:
		return true, v.validateChoiceRestriction(base, derived, baseOrder, derivedOrder)
	case base.Kind == modelSequence && derived.Kind == modelSequence:
		return true, v.validateOrderedGroupRestriction(base, derived, "sequence restriction is not subset of base")
	case base.Kind == modelSequence && derived.Kind == modelChoice:
		return true, contentRestrictionMismatch("choice restriction of sequence is forbidden")
	case base.Kind == modelAll && derived.Kind == modelAll:
		return true, v.validateOrderedGroupRestriction(base, derived, "all restriction is not subset of base")
	case base.Kind == modelAll && derived.Kind == modelSequence:
		return true, v.validateSequenceRestrictsAll(base, derived)
	case base.Kind == modelChoice && derived.Kind == modelSequence:
		return true, v.validateSequenceRestrictsChoice(base, derived)
	case base.Kind == modelSequence && derived.Kind == modelAll:
		return true, contentRestrictionMismatch("all restriction of sequence is forbidden")
	default:
		return false, nil
	}
}

func (v contentRestrictionValidator) choiceRestrictionBranchAllowed(base []contentParticle, derived contentParticle) (bool, error) {
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

func (v contentRestrictionValidator) validateChoiceRestriction(base, derived contentModelNode, baseOrder, derivedOrder restrictionChoiceOrder) error {
	if !occurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return contentRestrictionMismatch("choice restriction occurrence is not subset of base")
	}
	if !baseOrder.ordered() || !derivedOrder.ordered() {
		return v.validateChoiceSpans(base.Particles, derived.Particles, baseOrder, derivedOrder)
	}
	baseIndex := 0
	for _, derivedParticle := range derived.Particles {
		matched, err := v.matchChoiceRestrictionBranch(base.Particles, &baseIndex, derivedParticle)
		if err != nil {
			return err
		}
		if !matched {
			return contentRestrictionMismatch("choice restriction branch is not subset of base")
		}
	}
	return nil
}

func (v contentRestrictionValidator) matchChoiceRestrictionBranch(
	base []contentParticle,
	baseIndex *int,
	derived contentParticle,
) (bool, error) {
	for *baseIndex < len(base) {
		allowed, err := v.choiceBranchRestricts(base[*baseIndex], derived)
		if err != nil {
			return allowed, err
		}
		// RecurseLax requires each target to follow the preceding target.
		*baseIndex++
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

func (v contentRestrictionValidator) choiceBranchRestricts(base, derived contentParticle) (bool, error) {
	err := v.compareParticleRestriction(base, derived)
	if err == nil {
		return true, nil
	}
	if isContentRestrictionMismatch(err) {
		return false, nil
	}
	return false, err
}

func (v contentRestrictionValidator) validateOrderedGroupRestriction(base, derived contentModelNode, msg string) error {
	if !occurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return contentRestrictionMismatch(msg)
	}
	matcher := orderedGroupRestrictionMatcher{validator: v, base: base.Particles, message: msg}
	for _, derivedParticle := range derived.Particles {
		if err := matcher.match(derivedParticle); err != nil {
			return err
		}
	}
	return matcher.validateRemainder()
}

type orderedGroupRestrictionMatcher struct {
	validator contentRestrictionValidator
	message   string
	base      []contentParticle
	next      int
}

func (m *orderedGroupRestrictionMatcher) match(derived contentParticle) error {
	for m.next < len(m.base) {
		err := m.validator.compareParticleRestriction(m.base[m.next], derived)
		if err == nil {
			m.next++
			return nil
		}
		if !isContentRestrictionMismatch(err) {
			return err
		}
		emptiable, err := m.validator.analysis.ParticleEmptiable(m.base[m.next])
		if err != nil {
			return err
		}
		if !emptiable {
			return contentRestrictionMismatch(m.message)
		}
		m.next++
	}
	return contentRestrictionMismatch(m.message)
}

func (m *orderedGroupRestrictionMatcher) validateRemainder() error {
	for ; m.next < len(m.base); m.next++ {
		emptiable, err := m.validator.analysis.ParticleEmptiable(m.base[m.next])
		if err != nil {
			return err
		}
		if !emptiable {
			return contentRestrictionMismatch(m.message)
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateSequenceRestrictsAll(base, derived contentModelNode) error {
	if !occurrenceRangeSubset(derived.Occurs, base.Occurs) {
		return contentRestrictionMismatch("sequence restriction occurrence is not subset of all")
	}
	return v.validateMappedGroupRestriction(base, derived, "sequence restriction particle is not subset of all", "sequence restriction omits required all particle")
}

func (v contentRestrictionValidator) validateMappedGroupRestriction(base, derived contentModelNode, particleMsg, omittedMsg string) error {
	mapped := make([]bool, len(base.Particles))
	for _, derivedParticle := range derived.Particles {
		match, err := v.findMappedGroupParticle(base.Particles, mapped, derivedParticle)
		if err != nil {
			return err
		}
		if match < 0 {
			return contentRestrictionMismatch(particleMsg)
		}
		mapped[match] = true
	}
	return v.validateUnmappedGroupParticles(base.Particles, mapped, omittedMsg)
}

func (v contentRestrictionValidator) findMappedGroupParticle(
	base []contentParticle,
	mapped []bool,
	derived contentParticle,
) (int, error) {
	for i, baseParticle := range base {
		if mapped[i] {
			continue
		}
		err := v.compareParticleRestriction(baseParticle, derived)
		if err == nil {
			return i, nil
		}
		if !isContentRestrictionMismatch(err) {
			return -1, err
		}
	}
	return -1, nil
}

func (v contentRestrictionValidator) validateUnmappedGroupParticles(
	base []contentParticle,
	mapped []bool,
	message string,
) error {
	for i, baseParticle := range base {
		if mapped[i] {
			continue
		}
		emptiable, err := v.analysis.ParticleEmptiable(baseParticle)
		if err != nil {
			return err
		}
		if !emptiable {
			return contentRestrictionMismatch(message)
		}
	}
	return nil
}

func (v contentRestrictionValidator) validateSequenceRestrictsChoice(base, derived contentModelNode) error {
	if !occurrenceRangeSubset(sequenceChoiceRange(derived), base.Occurs) {
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

func (v contentRestrictionValidator) modelContainsWildcard(model contentModelNode) (bool, error) {
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

func (v contentRestrictionValidator) particleContainsWildcard(p contentParticle) (bool, error) {
	if err := v.charge(); err != nil {
		return false, err
	}
	switch p.Kind {
	case particleWildcard:
		return true, nil
	case particleModel:
		return v.modelIDContainsWildcard(p.Model)
	case particleElement:
		return false, nil
	default:
	}
	return false, nil
}

func (v contentRestrictionValidator) modelIDContainsWildcard(id contentModelID) (bool, error) {
	if result, ok := v.wildcards[id]; ok {
		return result, nil
	}
	model, err := v.requireContentModel(id)
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

func (v contentRestrictionValidator) compareParticleRestriction(base, derived contentParticle) error {
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
	if !occurrenceRangeSubset(derivedRange, baseRange) {
		return contentRestrictionMismatch("particle restriction occurrence is not subset of base")
	}
	if derived.Kind == particleModel {
		model, err := v.requireContentModel(derived.Model)
		if err != nil {
			return err
		}
		// Empty restrictions are admitted by the aggregate range check.
		if model.Kind == modelEmpty {
			return nil
		}
	}
	switch base.Kind {
	case particleWildcard:
		return v.validateParticleRestrictsWildcard(base, derived)
	case particleElement:
		return v.validateParticleRestrictsElement(base, derived)
	case particleModel:
		return v.validateParticleRestrictsModel(base, derived)
	default:
		return nil
	}
}

func (v contentRestrictionValidator) validateParticleRestrictsElement(base, derived contentParticle) error {
	if derived.Kind != particleElement {
		return contentRestrictionMismatch("particle restriction is not subset of element")
	}
	baseElement, derivedElement, err := v.restrictedElementIDs(base.Element, derived.Element)
	if err != nil {
		return err
	}
	baseDecl, derivedDecl, err := v.restrictedElementDeclarations(baseElement, derivedElement)
	if err != nil {
		return err
	}
	if baseDecl.Scope == declarationScopeGlobal && derivedDecl.Scope == declarationScopeGlobal {
		return nil
	}
	if err := v.validateRestrictedElementType(baseDecl, derivedDecl); err != nil {
		return err
	}
	return validateRestrictedElementProperties(baseDecl, derivedDecl)
}

func (v contentRestrictionValidator) restrictedElementIDs(base, derived ElementID) (baseID, derivedID ElementID, err error) {
	baseName, err := v.requireElementName(base)
	if err != nil {
		return NoElement, NoElement, err
	}
	derivedName, err := v.requireElementName(derived)
	if err != nil {
		return NoElement, NoElement, err
	}
	if baseName != derivedName {
		return NoElement, NoElement, contentRestrictionMismatch("element restriction name is not subset of base")
	}
	return base, derived, nil
}

func (v contentRestrictionValidator) restrictedElementDeclarations(
	base, derived ElementID,
) (baseDecl, derivedDecl particleRestrictionElement, err error) {
	baseDecl, err = v.elementRestriction(base)
	if err != nil {
		return particleRestrictionElement{}, particleRestrictionElement{}, err
	}
	derivedDecl, err = v.elementRestriction(derived)
	if err != nil {
		return particleRestrictionElement{}, particleRestrictionElement{}, err
	}
	if baseDecl.Scope == declarationScopeInvalid || derivedDecl.Scope == declarationScopeInvalid {
		return particleRestrictionElement{}, particleRestrictionElement{}, contentRestrictionInvariant("content restriction references element declaration with invalid scope")
	}
	return baseDecl, derivedDecl, nil
}

func (v contentRestrictionValidator) validateRestrictedElementType(
	base, derived particleRestrictionElement,
) error {
	const excluded = DerivationExtension | derivationList | derivationUnion
	mask, ok, err := deriveTypeMask(v.rt, derived.Type, base.Type, v.spend)
	if err != nil {
		return err
	}
	if !ok || mask&excluded != 0 {
		return contentRestrictionMismatch("element restriction type is not derived from base")
	}
	return nil
}

func validateRestrictedElementProperties(base, derived particleRestrictionElement) error {
	if derived.Nillable && !base.Nillable {
		return contentRestrictionMismatch("element restriction nillable is not subset of base")
	}
	if derived.Block&base.Block != base.Block {
		return contentRestrictionMismatch("element restriction block is not subset of base")
	}
	if !derived.Identities.IsSubsetOf(base.Identities) {
		return contentRestrictionMismatch("element restriction identity constraints are not subset of base")
	}
	if base.Fixed.Present && !fixedValueConstraintEqual(base.Fixed, derived.Fixed) {
		return contentRestrictionMismatch("element restriction fixed value is not subset of base")
	}
	return nil
}

func (v contentRestrictionValidator) validateParticleRestrictsModel(base, derived contentParticle) error {
	model, err := v.requireContentModel(base.Model)
	if err != nil {
		return err
	}
	if model.Kind == modelAny {
		return nil
	}
	model.Occurs = multiplyOccurrence(model.Occurs, base.Occurs)
	var restricted contentModelNode
	switch derived.Kind {
	case particleWildcard:
		return contentRestrictionMismatch("wildcard restriction is not subset of model group")
	case particleElement:
		// RecurseAsIfGroup preserves the element's bounds inside a unit group.
		restricted = contentModelNode{Kind: model.Kind, Occurs: occurrence{Min: 1, Max: 1}, Particles: []contentParticle{derived}}
	case particleModel:
		restricted, err = v.requireContentModel(derived.Model)
		if err != nil {
			return err
		}
		if restricted.Kind == modelEmpty {
			return nil
		}
		restricted.Occurs = multiplyOccurrence(restricted.Occurs, derived.Occurs)
	default:
		return contentRestrictionInvariant("content restriction references invalid particle kind")
	}
	var derivedOrder restrictionChoiceOrder
	if derived.Kind == particleModel {
		derivedOrder = v.choiceOrder(derived.Model)
	}
	if handled, err := v.validateKnownGroupRestriction(model, restricted, v.choiceOrder(base.Model), derivedOrder); handled || err != nil {
		return err
	}
	return v.validateFallbackContentRestriction(model, restricted)
}

func (v contentRestrictionValidator) validateParticleRestrictsWildcard(base, derived contentParticle) error {
	if err := v.charge(); err != nil {
		return err
	}
	switch derived.Kind {
	case particleElement:
		return v.validateElementRestrictsWildcard(base.Wildcard, derived.Element)
	case particleWildcard:
		return v.validateWildcardRestrictsWildcard(base.Wildcard, derived.Wildcard)
	case particleModel:
		return v.validateModelRestrictsWildcard(base, derived.Model)
	}
	return nil
}

func (v contentRestrictionValidator) validateElementRestrictsWildcard(base WildcardID, derived ElementID) error {
	baseWildcard, err := v.requireWildcard(base)
	if err != nil {
		return err
	}
	derivedName, err := v.requireElementName(derived)
	if err != nil {
		return err
	}
	if !wildcardAllowsNamespace(baseWildcard, derivedName.Namespace) {
		return contentRestrictionMismatch("element restriction is not allowed by wildcard")
	}
	return nil
}

func (v contentRestrictionValidator) validateWildcardRestrictsWildcard(base, derived WildcardID) error {
	derivedWildcard, err := v.requireWildcard(derived)
	if err != nil {
		return err
	}
	baseWildcard, err := v.requireWildcard(base)
	if err != nil {
		return err
	}
	if !wildcardSubset(derivedWildcard, baseWildcard) {
		return contentRestrictionMismatch("wildcard restriction is not subset of base")
	}
	return nil
}

func (v contentRestrictionValidator) validateModelRestrictsWildcard(base contentParticle, derived contentModelID) error {
	model, err := v.requireContentModel(derived)
	if err != nil {
		return err
	}
	// R-240 (I-004 in docs/spec/INTERPRETATIONS.md) applies the wildcard minimum to the group's effective range,
	// already checked by compareParticleRestriction, rather than each member.
	base.Occurs.Min = 0
	for _, child := range model.Particles {
		if err := v.compareParticleRestriction(base, child); err != nil {
			return err
		}
	}
	return nil
}

func (v contentRestrictionValidator) requireContentModel(id contentModelID) (contentModelNode, error) {
	model, ok := v.ContentModel(id)
	if !ok {
		return contentModelNode{}, contentRestrictionInvariant("content restriction references missing content model")
	}
	return model, nil
}

func (v contentRestrictionValidator) ContentModel(id contentModelID) (contentModelNode, bool) {
	if v.charge() != nil {
		return contentModelNode{}, false
	}
	return v.analysis.contentModel(id)
}

func (v contentRestrictionValidator) ElementName(id ElementID) (QName, bool) {
	if v.charge() != nil {
		return QName{}, false
	}
	return v.rt.ElementName(id)
}

func (v contentRestrictionValidator) Wildcard(id WildcardID) (wildcardRuntime, bool) {
	if v.charge() != nil {
		return wildcardRuntime{}, false
	}
	return wildcardByID(v.rt.Wildcards, id)
}

func (v contentRestrictionValidator) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	if v.charge() != nil {
		return NoElement, false
	}
	return v.rt.Substitutions.MemberByName(id, name)
}

func (v contentRestrictionValidator) requireElementName(id ElementID) (QName, error) {
	name, ok := v.ElementName(id)
	if !ok {
		if err := v.finish(nil); err != nil {
			return QName{}, err
		}
		return QName{}, contentRestrictionInvariant("content restriction references missing element name")
	}
	return name, nil
}

func (v contentRestrictionValidator) elementRestriction(id ElementID) (particleRestrictionElement, error) {
	if err := v.charge(); err != nil {
		return particleRestrictionElement{}, err
	}
	decl, ok := v.rt.ElementRestriction(id)
	if !ok {
		return particleRestrictionElement{}, contentRestrictionInvariant("content restriction references missing element declaration")
	}
	return decl, nil
}

func (v contentRestrictionValidator) requireWildcard(id WildcardID) (wildcardRuntime, error) {
	wildcard, ok := v.Wildcard(id)
	if !ok {
		return wildcardRuntime{}, contentRestrictionInvariant("content restriction references missing wildcard")
	}
	return wildcard, nil
}
