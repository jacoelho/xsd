package semanticcheck

import (
	"slices"

	"github.com/jacoelho/xsd/internal/model"
)

func normalizePointlessParticle(p model.Particle) model.Particle {
	for {
		mg, ok := p.(*model.ModelGroup)
		if !ok || mg == nil {
			return p
		}
		if !mg.MinOccurs.IsOne() || !mg.MaxOccurs.IsOne() {
			return p
		}
		children := derivationChildren(mg)
		if len(children) != 1 {
			return p
		}
		p = children[0]
	}
}

func derivationChildren(mg *model.ModelGroup) []model.Particle {
	if mg == nil {
		return nil
	}
	children := make([]model.Particle, 0, len(mg.Particles))
	for _, child := range mg.Particles {
		children = append(children, gatherPointlessChildren(mg.Kind, child)...)
	}
	return children
}

func gatherPointlessChildren(parentKind model.GroupKind, particle model.Particle) []model.Particle {
	switch p := particle.(type) {
	case *model.ElementDecl, *model.AnyElement:
		return []model.Particle{p}
	case *model.ModelGroup:
		if !p.MinOccurs.IsOne() || !p.MaxOccurs.IsOne() {
			return []model.Particle{p}
		}
		if len(p.Particles) == 1 {
			return gatherPointlessChildren(parentKind, p.Particles[0])
		}
		if p.Kind == parentKind {
			var out []model.Particle
			for _, child := range p.Particles {
				out = append(out, gatherPointlessChildren(parentKind, child)...)
			}
			return out
		}
		return []model.Particle{p}
	default:
		return []model.Particle{p}
	}
}

// isBlockSuperset checks if restrictionBlock is a superset of baseBlock.
// Restriction block must contain all derivation methods in base block
// (i.e., restriction cannot allow more than base).
func isBlockSuperset(restrictionBlock, baseBlock model.DerivationSet) bool {
	if baseBlock.Has(model.DerivationExtension) && !restrictionBlock.Has(model.DerivationExtension) {
		return false
	}
	if baseBlock.Has(model.DerivationRestriction) && !restrictionBlock.Has(model.DerivationRestriction) {
		return false
	}
	if baseBlock.Has(model.DerivationSubstitution) && !restrictionBlock.Has(model.DerivationSubstitution) {
		return false
	}
	return true
}

// calculateEffectiveOccurrence calculates the effective minOccurs and maxOccurs
// for a model group by considering the group's occurrence and its children.
// For sequences: effective = group.occ * sum(children.occ)
// For choices: effective = group.occ * max(children.occ) for max, group.occ * min(children.minOcc) for min
func calculateEffectiveOccurrence(mg *model.ModelGroup) (minOcc, maxOcc model.Occurs) {
	groupMinOcc := mg.MinOcc()
	groupMaxOcc := mg.MaxOcc()

	if len(mg.Particles) == 0 {
		return model.OccursFromInt(0), model.OccursFromInt(0)
	}

	switch mg.Kind {
	case model.Sequence:
		sumMinOcc := model.OccursFromInt(0)
		sumMaxOcc := model.OccursFromInt(0)
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			sumMinOcc = model.AddOccurs(sumMinOcc, childMin)
			sumMaxOcc = model.AddOccurs(sumMaxOcc, childMax)
		}
		minOcc = model.MulOccurs(groupMinOcc, sumMinOcc)
		maxOcc = model.MulOccurs(groupMaxOcc, sumMaxOcc)
	case model.Choice:
		childMinOcc := model.OccursFromInt(0)
		childMaxOcc := model.OccursFromInt(0)
		childMinOccSet := false
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			if childMax.IsZero() {
				continue
			}
			if !childMinOccSet || childMin.Cmp(childMinOcc) < 0 {
				childMinOcc = childMin
				childMinOccSet = true
			}
			childMaxOcc = model.MaxOccurs(childMaxOcc, childMax)
		}
		if !childMinOccSet {
			childMinOcc = model.OccursFromInt(0)
		}
		minOcc = model.MulOccurs(groupMinOcc, childMinOcc)
		maxOcc = model.MulOccurs(groupMaxOcc, childMaxOcc)
	case model.AllGroup:
		sumMinOcc := model.OccursFromInt(0)
		sumMaxOcc := model.OccursFromInt(0)
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			sumMinOcc = model.AddOccurs(sumMinOcc, childMin)
			sumMaxOcc = model.AddOccurs(sumMaxOcc, childMax)
		}
		minOcc = model.MulOccurs(groupMinOcc, sumMinOcc)
		maxOcc = model.MulOccurs(groupMaxOcc, sumMaxOcc)
	default:
		minOcc = groupMinOcc
		maxOcc = groupMaxOcc
	}
	return
}

// getParticleEffectiveOccurrence gets the effective occurrence of a single particle
func getParticleEffectiveOccurrence(p model.Particle) (minOcc, maxOcc model.Occurs) {
	switch particle := p.(type) {
	case *model.ModelGroup:
		return calculateEffectiveOccurrence(particle)
	case *model.ElementDecl:
		return particle.MinOcc(), particle.MaxOcc()
	case *model.AnyElement:
		return particle.MinOccurs, particle.MaxOccurs
	default:
		return p.MinOcc(), p.MaxOcc()
	}
}

// isEffectivelyOptional checks if a ModelGroup is effectively optional
// (all its particles are optional, making the group itself effectively optional)
func isEffectivelyOptional(mg *model.ModelGroup) bool {
	if len(mg.Particles) == 0 {
		return true
	}
	for _, particle := range mg.Particles {
		if particle.MinOcc().CmpInt(0) > 0 {
			return false
		}
		if nestedMG, ok := particle.(*model.ModelGroup); ok {
			if !isEffectivelyOptional(nestedMG) {
				return false
			}
		}
	}
	return true
}

// isEmptiableParticle reports whether a particle can match the empty sequence.
// Per XSD 1.0 Structures, a particle is emptiable if it can be satisfied without
// consuming any element information items.
func isEmptiableParticle(p model.Particle) bool {
	if p == nil {
		return true
	}
	if p.MaxOcc().IsZero() {
		return true
	}
	if p.MinOcc().IsZero() {
		return true
	}

	switch pt := p.(type) {
	case *model.ModelGroup:
		switch pt.Kind {
		case model.Sequence, model.AllGroup:
			for _, child := range pt.Particles {
				if !isEmptiableParticle(child) {
					return false
				}
			}
			return true
		case model.Choice:
			return slices.ContainsFunc(pt.Particles, isEmptiableParticle)
		default:
			return false
		}
	case *model.ElementDecl, *model.AnyElement:
		return false
	default:
		return p.MinOcc().IsZero()
	}
}
