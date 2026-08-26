package runtime

import "errors"

// ContentModelAnalysis is the bounded owner of content-model emptiability,
// count-range, substitution-name, and particle-overlap facts.
type ContentModelAnalysis struct {
	rt        ParticleRuntime
	work      ContentModelWork
	names     map[ElementID]particleAcceptedNames
	emptiable map[ContentModelID]bool
	ranges    map[ContentModelID]Occurrence
	emptyPath map[ContentModelID]bool
	rangePath map[ContentModelID]bool
}

type particleAcceptedNames struct {
	byName  map[QName]ElementID
	ordered []QName
}

// NewContentModelAnalysis creates a reusable bounded analysis context.
func NewContentModelAnalysis(rt ParticleRuntime, work ContentModelWork) (*ContentModelAnalysis, error) {
	if rt == nil {
		return nil, errors.New("content model analysis runtime is nil")
	}
	if err := requireContentModelWork(work); err != nil {
		return nil, err
	}
	return &ContentModelAnalysis{
		rt:        rt,
		work:      work,
		names:     make(map[ElementID]particleAcceptedNames),
		emptiable: make(map[ContentModelID]bool),
		ranges:    make(map[ContentModelID]Occurrence),
		emptyPath: make(map[ContentModelID]bool),
		rangePath: make(map[ContentModelID]bool),
	}, nil
}

// ParticleEmptiable reports whether p accepts an empty sequence, charging and
// memoizing every nested-model derivation.
func (a *ContentModelAnalysis) ParticleEmptiable(p Particle) (bool, error) {
	return a.particleEmptiable(p)
}

// ModelEmptiable reports whether a model accepts an empty sequence, charging
// and memoizing every nested-model derivation.
func (a *ContentModelAnalysis) ModelEmptiable(id ContentModelID) (bool, error) {
	if id == NoContentModel {
		return true, nil
	}
	return a.modelEmptiable(id)
}

// ModelCountRange derives the number of elements a model can consume,
// charging and memoizing every nested-model derivation.
func (a *ContentModelAnalysis) ModelCountRange(id ContentModelID) (Occurrence, error) {
	if id == NoContentModel {
		return Occurrence{}, nil
	}
	return a.modelCountRange(id)
}

// ParticleCountRange derives the number of elements p can consume, charging
// and memoizing every nested-model derivation.
func (a *ContentModelAnalysis) ParticleCountRange(p Particle) (Occurrence, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return Occurrence{}, err
	}
	var term Occurrence
	switch p.Kind {
	case ParticleElement, ParticleWildcard:
		term = Occurrence{Min: 1, Max: 1}
	case ParticleModel:
		var err error
		term, err = a.ModelCountRange(p.Model)
		if err != nil {
			return Occurrence{}, err
		}
	}
	return MultiplyOccurrence(term, p.Occurs), nil
}

func (a *ContentModelAnalysis) modelCountRange(id ContentModelID) (Occurrence, error) {
	if result, ok := a.ranges[id]; ok {
		return result, nil
	}
	if a.rangePath[id] {
		return Occurrence{}, errors.New("content model count analysis contains a cycle")
	}
	if err := spendContentModelWork(a.work); err != nil {
		return Occurrence{}, err
	}
	model, ok := a.rt.ContentModel(id)
	if !ok {
		return Occurrence{}, errors.New("content model count analysis references missing content model")
	}
	a.rangePath[id] = true
	defer delete(a.rangePath, id)
	var term Occurrence
	switch model.Kind {
	case ModelEmpty:
	case ModelAny:
		term = Occurrence{Unbounded: true}
	case ModelSequence, ModelAll:
		for _, particle := range model.Particles {
			child, err := a.ParticleCountRange(particle)
			if err != nil {
				return Occurrence{}, err
			}
			term = AddOccurrenceRanges(term, child)
		}
	case ModelChoice:
		for i, particle := range model.Particles {
			child, err := a.ParticleCountRange(particle)
			if err != nil {
				return Occurrence{}, err
			}
			if i == 0 {
				term = child
			} else {
				term = UnionOccurrenceRanges(term, child)
			}
		}
	}
	result := MultiplyOccurrence(term, model.Occurs)
	a.ranges[id] = result
	return result, nil
}

// Overlap reports one element name accepted by both particles.
func (a *ContentModelAnalysis) Overlap(left, right Particle) (QName, bool, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return QName{}, false, err
	}
	if left.Kind == ParticleModel {
		return a.modelStartOverlap(left.Model, right)
	}
	if right.Kind == ParticleModel {
		return a.modelStartOverlap(right.Model, left)
	}
	if left.Kind == ParticleWildcard && right.Kind == ParticleWildcard {
		leftWildcard, leftOK := a.rt.Wildcard(left.Wildcard)
		rightWildcard, rightOK := a.rt.Wildcard(right.Wildcard)
		return QName{}, leftOK && rightOK && WildcardsOverlap(leftWildcard, rightWildcard), nil
	}
	if left.Kind == ParticleElement {
		if name, ok, err := a.elementOverlap(left.Element, right); ok || err != nil {
			return name, ok, err
		}
	}
	if right.Kind == ParticleElement {
		return a.elementOverlap(right.Element, left)
	}
	return QName{}, false, nil
}

func (a *ContentModelAnalysis) modelStartOverlap(id ContentModelID, particle Particle) (QName, bool, error) {
	model, ok := a.rt.ContentModel(id)
	if !ok {
		return QName{}, false, errors.New("content model overlap analysis references missing content model")
	}
	switch model.Kind {
	case ModelAll, ModelChoice, ModelSequence:
	default:
		return QName{}, false, nil
	}
	for _, child := range model.Particles {
		if err := spendContentModelWork(a.work); err != nil {
			return QName{}, false, err
		}
		if name, overlap, err := a.Overlap(child, particle); overlap || err != nil {
			return name, overlap, err
		}
		if model.Kind == ModelSequence {
			emptiable, err := a.particleEmptiable(child)
			if err != nil {
				return QName{}, false, err
			}
			if !emptiable {
				break
			}
		}
	}
	return QName{}, false, nil
}

func (a *ContentModelAnalysis) elementOverlap(id ElementID, particle Particle) (QName, bool, error) {
	accepted, ok, err := a.acceptedNames(id)
	if err != nil || !ok {
		return QName{}, false, err
	}
	switch particle.Kind {
	case ParticleElement:
		other, otherOK, otherErr := a.acceptedNames(particle.Element)
		if otherErr != nil || !otherOK {
			return QName{}, false, otherErr
		}
		for _, name := range accepted.ordered {
			if err := spendContentModelWork(a.work); err != nil {
				return QName{}, false, err
			}
			if _, exists := other.byName[name]; exists {
				return name, true, nil
			}
		}
	case ParticleWildcard:
		wildcard, wildcardOK := a.rt.Wildcard(particle.Wildcard)
		if !wildcardOK {
			return QName{}, false, nil
		}
		for _, name := range accepted.ordered {
			if err := spendContentModelWork(a.work); err != nil {
				return QName{}, false, err
			}
			if WildcardAllowsNamespace(wildcard, name.Namespace) {
				return name, true, nil
			}
		}
	case ParticleModel:
	}
	return QName{}, false, nil
}

func (a *ContentModelAnalysis) acceptedNames(id ElementID) (particleAcceptedNames, bool, error) {
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
	var workErr error
	a.rt.ForEachSubstitutionMember(id, func(member ElementID) bool {
		workErr = spendContentModelWork(a.work)
		if workErr != nil {
			return false
		}
		memberName, memberOK := a.rt.ElementName(member)
		if !memberOK {
			return true
		}
		allowed, allowedOK := a.rt.SubstitutionMemberByName(id, memberName)
		if !allowedOK || allowed != member {
			return true
		}
		if _, exists := accepted.byName[memberName]; !exists {
			accepted.ordered = append(accepted.ordered, memberName)
			accepted.byName[memberName] = member
		}
		return true
	})
	if workErr != nil {
		return particleAcceptedNames{}, false, workErr
	}
	a.names[id] = accepted
	return accepted, true, nil
}

func (a *ContentModelAnalysis) particleEmptiable(particle Particle) (bool, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return false, err
	}
	if particle.Occurs.Min == 0 {
		return true, nil
	}
	if particle.Kind != ParticleModel {
		return false, nil
	}
	return a.ModelEmptiable(particle.Model)
}

func (a *ContentModelAnalysis) modelEmptiable(id ContentModelID) (bool, error) {
	if result, ok := a.emptiable[id]; ok {
		return result, nil
	}
	if a.emptyPath[id] {
		return false, errors.New("content model emptiability analysis contains a cycle")
	}
	model, ok := a.rt.ContentModel(id)
	if !ok {
		return false, errors.New("content model emptiability analysis references missing content model")
	}
	if err := spendContentModelWork(a.work); err != nil {
		return false, err
	}
	a.emptyPath[id] = true
	defer delete(a.emptyPath, id)
	result := model.Occurs.Min == 0 || model.Kind == ModelEmpty || model.Kind == ModelAny
	if !result {
		switch model.Kind {
		case ModelEmpty, ModelAny:
		case ModelSequence, ModelAll:
			result = true
			for _, particle := range model.Particles {
				emptiable, err := a.particleEmptiable(particle)
				if err != nil {
					return false, err
				}
				if !emptiable {
					result = false
					break
				}
			}
		case ModelChoice:
			for _, particle := range model.Particles {
				emptiable, err := a.particleEmptiable(particle)
				if err != nil {
					return false, err
				}
				if emptiable {
					result = true
					break
				}
			}
		}
	}
	a.emptiable[id] = result
	return result, nil
}
