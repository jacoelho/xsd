package runtime

import "slices"

// ContentModelRuntime supplies content models for model-shape algorithms.
type ContentModelRuntime interface {
	ContentModel(id ContentModelID) (ContentModel, bool)
}

// ParticleRuntime supplies read-only runtime tables needed for particle
// matching semantics.
type ParticleRuntime interface {
	ContentModelRuntime
	ElementName(id ElementID) (QName, bool)
	Wildcard(id WildcardID) (Wildcard, bool)
	ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool)
	SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool)
}

// ModelEmptiable reports whether modelID can match an empty sequence.
func ModelEmptiable(rt ContentModelRuntime, modelID ContentModelID) bool {
	if modelID == NoContentModel {
		return true
	}
	model, ok := rt.ContentModel(modelID)
	if !ok {
		return false
	}
	if model.Occurs.Min == 0 {
		return true
	}
	switch model.Kind {
	case ModelEmpty, ModelAny:
		return true
	case ModelSequence, ModelAll:
		for _, p := range model.Particles {
			if !ParticleEmptiable(rt, p) {
				return false
			}
		}
		return true
	case ModelChoice:
		return slices.ContainsFunc(model.Particles, func(p Particle) bool {
			return ParticleEmptiable(rt, p)
		})
	default:
		return false
	}
}

// ModelHasNoParticles reports whether modelID is structurally empty.
func ModelHasNoParticles(rt ContentModelRuntime, modelID ContentModelID) bool {
	if modelID == NoContentModel {
		return true
	}
	model, ok := rt.ContentModel(modelID)
	if !ok {
		return false
	}
	switch model.Kind {
	case ModelEmpty:
		return true
	case ModelSequence, ModelChoice, ModelAll:
		return len(model.Particles) == 0
	default:
		return false
	}
}

// ParticleEmptiable reports whether p can match an empty sequence.
func ParticleEmptiable(rt ContentModelRuntime, p Particle) bool {
	if p.Occurs.Min == 0 {
		return true
	}
	if p.Kind == ParticleModel {
		return ModelEmptiable(rt, p.Model)
	}
	return false
}

// ParticleEffectiveMin reports the effective minimum occurrence for p after
// accounting for an emptiable nested model.
func ParticleEffectiveMin(rt ContentModelRuntime, p Particle) uint32 {
	if p.Kind == ParticleModel && ModelEmptiable(rt, p.Model) {
		return 0
	}
	return p.Occurs.Min
}

// ModelCountRange reports the minimum and maximum element count a model can consume.
func ModelCountRange(rt ContentModelRuntime, modelID ContentModelID) Occurrence {
	if modelID == NoContentModel {
		return Occurrence{}
	}
	model, ok := rt.ContentModel(modelID)
	if !ok {
		return Occurrence{}
	}
	var term Occurrence
	switch model.Kind {
	case ModelEmpty:
		term = Occurrence{}
	case ModelAny:
		term = Occurrence{Min: 0, Unbounded: true}
	case ModelSequence, ModelAll:
		for _, p := range model.Particles {
			term = AddOccurrenceRanges(term, ParticleCountRange(rt, p))
		}
	case ModelChoice:
		if len(model.Particles) == 0 {
			term = Occurrence{}
			break
		}
		term = ParticleCountRange(rt, model.Particles[0])
		for _, p := range model.Particles[1:] {
			term = UnionOccurrenceRanges(term, ParticleCountRange(rt, p))
		}
	}
	return MultiplyOccurrence(term, model.Occurs)
}

// ParticleCountRange reports the minimum and maximum element count p can consume.
func ParticleCountRange(rt ContentModelRuntime, p Particle) Occurrence {
	var term Occurrence
	switch p.Kind {
	case ParticleElement, ParticleWildcard:
		term = Occurrence{Min: 1, Max: 1}
	case ParticleModel:
		term = ModelCountRange(rt, p.Model)
	default:
		term = Occurrence{}
	}
	return MultiplyOccurrence(term, p.Occurs)
}

// SequenceChoiceRange reports the range for a sequence that restricts a choice.
func SequenceChoiceRange(model ContentModel) Occurrence {
	particleCount := saturatingUint32(len(model.Particles))
	if model.Occurs.Unbounded {
		return Occurrence{Min: saturatingMul(model.Occurs.Min, particleCount), Unbounded: true}
	}
	return Occurrence{Min: saturatingMul(model.Occurs.Min, particleCount), Max: saturatingMul(model.Occurs.Max, particleCount)}
}

// AddOccurrenceRanges saturates sequence ranges before applying occurrence limits.
func AddOccurrenceRanges(a, b Occurrence) Occurrence {
	return Occurrence{Min: saturatingAdd(a.Min, b.Min), Max: saturatingAdd(a.Max, b.Max), Unbounded: a.Unbounded || b.Unbounded}
}

// UnionOccurrenceRanges returns the range accepted by either occurrence range.
func UnionOccurrenceRanges(a, b Occurrence) Occurrence {
	minOccurs := min(b.Min, a.Min)
	if a.Unbounded || b.Unbounded {
		return Occurrence{Min: minOccurs, Unbounded: true}
	}
	maxOccurs := max(b.Max, a.Max)
	return Occurrence{Min: minOccurs, Max: maxOccurs}
}

// MultiplyOccurrence applies occurrence constraints to an accepted range.
func MultiplyOccurrence(a, b Occurrence) Occurrence {
	minOccurs := saturatingMul(a.Min, b.Min)
	if a.Unbounded || b.Unbounded {
		return Occurrence{Min: minOccurs, Unbounded: true}
	}
	return Occurrence{Min: minOccurs, Max: saturatingMul(a.Max, b.Max)}
}

// OccurrenceRangeSubset reports whether derived is a subset of base.
func OccurrenceRangeSubset(derived, base Occurrence) bool {
	if derived.Min < base.Min {
		return false
	}
	if base.Unbounded {
		return true
	}
	if derived.Unbounded {
		return false
	}
	return derived.Max <= base.Max
}

func saturatingUint32(n int) uint32 {
	if n < 0 || uint64(n) > uint64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(n)
}

func saturatingAdd(a, b uint32) uint32 {
	if ^uint32(0)-a < b {
		return ^uint32(0)
	}
	return a + b
}

func saturatingMul(a, b uint32) uint32 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > ^uint32(0)/b {
		return ^uint32(0)
	}
	return a * b
}

// ParticlesOverlap reports whether a and b can consume the same element name.
func ParticlesOverlap(rt ParticleRuntime, a, b Particle) (QName, bool) {
	if a.Kind == ParticleModel {
		model, ok := rt.ContentModel(a.Model)
		if !ok {
			return QName{}, false
		}
		return modelStartOverlap(rt, model, b)
	}
	if b.Kind == ParticleModel {
		model, ok := rt.ContentModel(b.Model)
		if !ok {
			return QName{}, false
		}
		return modelStartOverlap(rt, model, a)
	}
	if a.Kind == ParticleWildcard && b.Kind == ParticleWildcard {
		wa, ok := rt.Wildcard(a.Wildcard)
		if !ok {
			return QName{}, false
		}
		wb, ok := rt.Wildcard(b.Wildcard)
		if !ok {
			return QName{}, false
		}
		return QName{}, WildcardsOverlap(wa, wb)
	}
	if name, ok := firstParticleElementNameMatchedBy(rt, a, b); ok {
		return name, true
	}
	if name, ok := firstParticleElementNameMatchedBy(rt, b, a); ok {
		return name, true
	}
	return QName{}, false
}

func modelStartOverlap(rt ParticleRuntime, model ContentModel, p Particle) (QName, bool) {
	switch model.Kind {
	case ModelAll, ModelChoice, ModelSequence:
	default:
		return QName{}, false
	}
	for _, child := range model.Particles {
		if name, ok := ParticlesOverlap(rt, child, p); ok {
			return name, true
		}
		if model.Kind == ModelSequence && !ParticleEmptiable(rt, child) {
			break
		}
	}
	return QName{}, false
}

// ParticleMatchesName reports whether p can consume name.
func ParticleMatchesName(rt ParticleRuntime, p Particle, name QName) bool {
	switch p.Kind {
	case ParticleElement:
		return elementParticleMatchesName(rt, p.Element, name)
	case ParticleWildcard:
		w, ok := rt.Wildcard(p.Wildcard)
		return ok && WildcardAllowsNamespace(w, name.Namespace)
	case ParticleModel:
		model, ok := rt.ContentModel(p.Model)
		return ok && modelStartMatchesName(rt, model, name)
	default:
		return false
	}
}

func firstParticleElementNameMatchedBy(rt ParticleRuntime, src, dst Particle) (QName, bool) {
	if src.Kind != ParticleElement {
		return QName{}, false
	}
	name, ok := rt.ElementName(src.Element)
	if !ok {
		return QName{}, false
	}
	if ParticleMatchesName(rt, dst, name) {
		return name, true
	}
	var found QName
	var matched bool
	rt.ForEachSubstitutionMember(src.Element, func(member ElementID) bool {
		name, ok := rt.ElementName(member)
		if !ok {
			return true
		}
		if allowed, ok := rt.SubstitutionMemberByName(src.Element, name); ok && allowed == member && ParticleMatchesName(rt, dst, name) {
			found = name
			matched = true
			return false
		}
		return true
	})
	return found, matched
}

func elementParticleMatchesName(rt ParticleRuntime, id ElementID, name QName) bool {
	elementName, ok := rt.ElementName(id)
	if !ok {
		return false
	}
	if elementName == name {
		return true
	}
	var matched bool
	rt.ForEachSubstitutionMember(id, func(member ElementID) bool {
		memberName, ok := rt.ElementName(member)
		if !ok {
			return true
		}
		if memberName == name {
			allowed, ok := rt.SubstitutionMemberByName(id, name)
			matched = ok && allowed == member
			return false
		}
		return true
	})
	return matched
}

func modelStartMatchesName(rt ParticleRuntime, model ContentModel, name QName) bool {
	switch model.Kind {
	case ModelAll, ModelChoice:
		for _, p := range model.Particles {
			if ParticleMatchesName(rt, p, name) {
				return true
			}
		}
	case ModelSequence:
		for _, p := range model.Particles {
			if ParticleMatchesName(rt, p, name) {
				return true
			}
			if !ParticleEmptiable(rt, p) {
				break
			}
		}
	default:
	}
	return false
}
