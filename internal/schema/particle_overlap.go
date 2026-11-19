package schema

import (
	"errors"
	"slices"
)

// contentModelAnalysis is the bounded owner of content-model emptiability,
// count-range, substitution-name, and particle-overlap facts.
type contentModelAnalysis struct {
	rt        *schemaBuild
	models    *restrictionModelView
	work      contentModelWork
	names     map[ElementID]particleAcceptedNames
	emptiable map[contentModelID]bool
	ranges    map[contentModelID]countRange
	emptyPath map[contentModelID]bool
	rangePath map[contentModelID]bool
}

func (a *contentModelAnalysis) contentModel(id contentModelID) (contentModelNode, bool) {
	if a.models != nil {
		return a.models.model(id)
	}
	if !validContentModelID(id, len(a.rt.Models)) {
		return contentModelNode{}, false
	}
	// Analysis borrows model children without mutation, just like its private
	// restriction view; defensive copies belong to component read boundaries.
	return a.rt.Models[id], true
}

// particleAcceptedNames holds the names an element particle accepts: its own
// and those of its effective substitution members. ordered is sorted by
// expanded name, so the name a Unique Particle Attribution diagnostic reports
// is the first overlapping name in that order.
type particleAcceptedNames struct {
	byName  map[QName]ElementID
	ordered []QName
}

// newContentModelAnalysis creates a reusable bounded analysis context.
func newContentModelAnalysis(rt *schemaBuild, work contentModelWork) (*contentModelAnalysis, error) {
	if rt == nil {
		return nil, errors.New("content model analysis runtime is nil")
	}
	if err := requireContentModelWork(work); err != nil {
		return nil, err
	}
	return &contentModelAnalysis{
		rt:        rt,
		work:      work,
		names:     make(map[ElementID]particleAcceptedNames),
		emptiable: make(map[contentModelID]bool),
		ranges:    make(map[contentModelID]countRange),
		emptyPath: make(map[contentModelID]bool),
		rangePath: make(map[contentModelID]bool),
	}, nil
}

// ParticleEmptiable reports whether p accepts an empty sequence, charging and
// memoizing every nested-model derivation.
func (a *contentModelAnalysis) ParticleEmptiable(p contentParticle) (bool, error) {
	return a.cachedParticleEmptiable(p)
}

// ModelEmptiable reports whether a model accepts an empty sequence, charging
// and memoizing every nested-model derivation.
func (a *contentModelAnalysis) ModelEmptiable(id contentModelID) (bool, error) {
	if id == noContentModel {
		return true, nil
	}
	return a.cachedModelEmptiable(id)
}

// countRange is the number of elements in the sequences a term matches. A
// term that matches no sequence, such as a choice that must occur and has no
// member that matches one (I-042 in docs/spec/INTERPRETATIONS.md), has no
// count: it adds no alternative to a choice, and a sequence that must hold it
// matches nothing.
type countRange struct {
	counts  occurrence
	nothing bool
}

// subsetOf reports whether every count of r is a count of base.
func (r countRange) subsetOf(base countRange) bool {
	if r.nothing {
		return true
	}
	return !base.nothing && occurrenceRangeSubset(r.counts, base.counts)
}

func (r countRange) emptiable() bool {
	return !r.nothing && r.counts.Min == 0
}

func (r countRange) repeated(occurs occurrence) countRange {
	if r.nothing {
		return countRange{nothing: occurs.Min != 0}
	}
	return countRange{counts: multiplyOccurrence(r.counts, occurs)}
}

// ModelCountRange derives the number of elements a model can consume,
// charging and memoizing every nested-model derivation.
func (a *contentModelAnalysis) ModelCountRange(id contentModelID) (countRange, error) {
	if id == noContentModel {
		return countRange{}, nil
	}
	return a.cachedModelCountRange(id)
}

// ParticleCountRange derives the number of elements p can consume, charging
// and memoizing every nested-model derivation.
func (a *contentModelAnalysis) ParticleCountRange(p contentParticle) (countRange, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return countRange{}, err
	}
	var term countRange
	switch p.Kind {
	case particleElement, particleWildcard:
		term = countRange{counts: occurrence{Min: 1, Max: 1}}
	case particleModel:
		var err error
		term, err = a.ModelCountRange(p.Model)
		if err != nil {
			return countRange{}, err
		}
	}
	return term.repeated(p.Occurs), nil
}

func (a *contentModelAnalysis) cachedModelCountRange(id contentModelID) (countRange, error) {
	if result, ok := a.ranges[id]; ok {
		return result, nil
	}
	if a.rangePath[id] {
		return countRange{}, errors.New("content model count analysis contains a cycle")
	}
	if err := spendContentModelWork(a.work); err != nil {
		return countRange{}, err
	}
	model, ok := a.contentModel(id)
	if !ok {
		return countRange{}, errors.New("content model count analysis references missing content model")
	}
	a.rangePath[id] = true
	defer delete(a.rangePath, id)
	term, err := a.modelTerm(id, model)
	if err != nil {
		return countRange{}, err
	}
	result := term.repeated(model.Occurs)
	a.ranges[id] = result
	return result, nil
}

func (a *contentModelAnalysis) modelTerm(id contentModelID, model contentModelNode) (countRange, error) {
	if a.models != nil {
		if term, ok := a.models.term(id); ok {
			return term, nil
		}
	}
	return a.modelTermCountRange(model)
}

func (a *contentModelAnalysis) modelTermCountRange(model contentModelNode) (countRange, error) {
	switch model.Kind {
	case modelEmpty:
		return countRange{}, nil
	case modelSequence, modelAll:
		return a.sequenceCountRange(model.Particles)
	case modelChoice:
		return a.choiceCountRange(model.Particles)
	}
	return countRange{}, nil
}

func (a *contentModelAnalysis) sequenceCountRange(particles []contentParticle) (countRange, error) {
	var result countRange
	for _, particle := range particles {
		child, err := a.ParticleCountRange(particle)
		if err != nil {
			return countRange{}, err
		}
		result.nothing = result.nothing || child.nothing
		result.counts = addOccurrenceRanges(result.counts, child.counts)
	}
	return result, nil
}

func (a *contentModelAnalysis) choiceCountRange(particles []contentParticle) (countRange, error) {
	result := countRange{nothing: true}
	for _, particle := range particles {
		child, err := a.ParticleCountRange(particle)
		if err != nil {
			return countRange{}, err
		}
		switch {
		case child.nothing:
		case result.nothing:
			result = child
		default:
			result.counts = unionOccurrenceRanges(result.counts, child.counts)
		}
	}
	return result, nil
}

// Overlap reports one element name accepted by both particles.
func (a *contentModelAnalysis) Overlap(left, right contentParticle) (QName, bool, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return QName{}, false, err
	}
	switch {
	case left.Kind == particleModel:
		return a.modelStartOverlap(left.Model, right)
	case right.Kind == particleModel:
		return a.modelStartOverlap(right.Model, left)
	case left.Kind == particleWildcard && right.Kind == particleWildcard:
		return a.wildcardOverlap(left.Wildcard, right.Wildcard)
	default:
		return a.elementParticleOverlap(left, right)
	}
}

func (a *contentModelAnalysis) wildcardOverlap(left, right WildcardID) (QName, bool, error) {
	leftWildcard, leftOK := wildcardByID(a.rt.Wildcards, left)
	rightWildcard, rightOK := wildcardByID(a.rt.Wildcards, right)
	return QName{}, leftOK && rightOK && wildcardsOverlap(leftWildcard, rightWildcard), nil
}

func (a *contentModelAnalysis) elementParticleOverlap(left, right contentParticle) (QName, bool, error) {
	if left.Kind == particleElement {
		if name, ok, err := a.elementOverlap(left.Element, right); ok || err != nil {
			return name, ok, err
		}
	}
	if right.Kind == particleElement {
		return a.elementOverlap(right.Element, left)
	}
	return QName{}, false, nil
}

func (a *contentModelAnalysis) modelStartOverlap(id contentModelID, particle contentParticle) (QName, bool, error) {
	model, ok := a.contentModel(id)
	if !ok {
		return QName{}, false, errors.New("content model overlap analysis references missing content model")
	}
	switch model.Kind {
	case modelAll, modelChoice, modelSequence:
	case modelEmpty:
		return QName{}, false, nil
	default:
		overlaps := false
		return QName{}, overlaps, nil
	}
	for _, child := range model.Particles {
		match, err := a.modelStartParticleOverlap(model.Kind, child, particle)
		if match.overlap || err != nil {
			return match.name, match.overlap, err
		}
		if match.stop {
			break
		}
	}
	return QName{}, false, nil
}

type modelStartParticleMatch struct {
	name    QName
	overlap bool
	stop    bool
}

func (a *contentModelAnalysis) modelStartParticleOverlap(
	kind modelKind,
	child, particle contentParticle,
) (modelStartParticleMatch, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return modelStartParticleMatch{}, err
	}
	name, overlap, err := a.Overlap(child, particle)
	if overlap || err != nil || kind != modelSequence {
		return modelStartParticleMatch{name: name, overlap: overlap}, err
	}
	emptiable, err := a.cachedParticleEmptiable(child)
	return modelStartParticleMatch{stop: !emptiable}, err
}

func (a *contentModelAnalysis) elementOverlap(id ElementID, particle contentParticle) (QName, bool, error) {
	accepted, ok, err := a.acceptedNames(id)
	if err != nil || !ok {
		return QName{}, false, err
	}
	switch particle.Kind {
	case particleElement:
		return a.acceptedElementOverlap(accepted, particle.Element)
	case particleWildcard:
		return a.acceptedWildcardOverlap(accepted, particle.Wildcard)
	case particleModel:
	}
	return QName{}, false, nil
}

func (a *contentModelAnalysis) acceptedElementOverlap(accepted particleAcceptedNames, id ElementID) (QName, bool, error) {
	other, ok, err := a.acceptedNames(id)
	if err != nil || !ok {
		return QName{}, false, err
	}
	for _, name := range accepted.ordered {
		if err := spendContentModelWork(a.work); err != nil {
			return QName{}, false, err
		}
		if _, exists := other.byName[name]; exists {
			return name, true, nil
		}
	}
	return QName{}, false, nil
}

func (a *contentModelAnalysis) acceptedWildcardOverlap(accepted particleAcceptedNames, id WildcardID) (QName, bool, error) {
	wildcard, ok := wildcardByID(a.rt.Wildcards, id)
	if !ok {
		return QName{}, false, nil
	}
	for _, name := range accepted.ordered {
		if err := spendContentModelWork(a.work); err != nil {
			return QName{}, false, err
		}
		if wildcardAllowsNamespace(wildcard, name.Namespace) {
			return name, true, nil
		}
	}
	return QName{}, false, nil
}

func (a *contentModelAnalysis) acceptedNames(id ElementID) (particleAcceptedNames, bool, error) {
	if accepted, ok := a.names[id]; ok {
		return accepted, true, nil
	}
	name, ok := a.rt.ElementName(id)
	if !ok {
		return particleAcceptedNames{}, false, nil
	}
	if err := spendContentModelWork(a.work); err != nil {
		return particleAcceptedNames{}, false, err
	}
	accepted := particleAcceptedNames{
		ordered: []QName{name},
		byName:  map[QName]ElementID{name: id},
	}
	collector := particleAcceptedNameCollector{analysis: a, head: id, accepted: &accepted}
	a.rt.Substitutions.ForEachMember(id, collector.collect)
	if collector.err != nil {
		return particleAcceptedNames{}, false, collector.err
	}
	members := accepted.ordered[1:]
	at, _ := slices.BinarySearchFunc(members, name, func(member, head QName) int {
		return compareQNameText(&a.rt.Names, member, head)
	})
	copy(accepted.ordered, members[:at])
	accepted.ordered[at] = name
	a.names[id] = accepted
	return accepted, true, nil
}

type particleAcceptedNameCollector struct {
	err      error
	analysis *contentModelAnalysis
	accepted *particleAcceptedNames
	head     ElementID
}

func (c *particleAcceptedNameCollector) collect(member ElementID) bool {
	c.err = spendContentModelWork(c.analysis.work)
	if c.err != nil {
		return false
	}
	memberName, ok := c.analysis.rt.ElementName(member)
	if !ok {
		return true
	}
	allowed, ok := c.analysis.rt.Substitutions.MemberByName(c.head, memberName)
	if !ok || allowed != member {
		return true
	}
	if _, exists := c.accepted.byName[memberName]; !exists {
		c.accepted.ordered = append(c.accepted.ordered, memberName)
		c.accepted.byName[memberName] = member
	}
	return true
}

func (a *contentModelAnalysis) cachedParticleEmptiable(particle contentParticle) (bool, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return false, err
	}
	if particle.Occurs.Min == 0 {
		return true, nil
	}
	if particle.Kind != particleModel {
		return false, nil
	}
	return a.ModelEmptiable(particle.Model)
}

func (a *contentModelAnalysis) cachedModelEmptiable(id contentModelID) (bool, error) {
	if result, ok := a.emptiable[id]; ok {
		return result, nil
	}
	if a.emptyPath[id] {
		return false, errors.New("content model emptiability analysis contains a cycle")
	}
	model, ok := a.contentModel(id)
	if !ok {
		return false, errors.New("content model emptiability analysis references missing content model")
	}
	if err := spendContentModelWork(a.work); err != nil {
		return false, err
	}
	a.emptyPath[id] = true
	defer delete(a.emptyPath, id)
	result, err := a.emptiableModel(id, model)
	if err != nil {
		return false, err
	}
	a.emptiable[id] = result
	return result, nil
}

func (a *contentModelAnalysis) emptiableModel(id contentModelID, model contentModelNode) (bool, error) {
	if a.models != nil {
		if term, ok := a.models.term(id); ok {
			return term.repeated(model.Occurs).emptiable(), nil
		}
	}
	return a.modelEmptiableValue(model)
}

func (a *contentModelAnalysis) modelEmptiableValue(model contentModelNode) (bool, error) {
	if model.Occurs.Min == 0 || model.Kind == modelEmpty {
		return true, nil
	}
	switch model.Kind {
	case modelSequence, modelAll:
		return a.allParticlesEmptiable(model.Particles)
	case modelChoice:
		return a.anyParticleEmptiable(model.Particles)
	case modelEmpty:
		return false, nil
	default:
	}
	return false, nil
}

func (a *contentModelAnalysis) allParticlesEmptiable(particles []contentParticle) (bool, error) {
	for _, particle := range particles {
		emptiable, err := a.cachedParticleEmptiable(particle)
		if err != nil {
			return false, err
		}
		if !emptiable {
			return false, nil
		}
	}
	return true, nil
}

func (a *contentModelAnalysis) anyParticleEmptiable(particles []contentParticle) (bool, error) {
	for _, particle := range particles {
		emptiable, err := a.cachedParticleEmptiable(particle)
		if err != nil {
			return false, err
		}
		if emptiable {
			return true, nil
		}
	}
	return false, nil
}
