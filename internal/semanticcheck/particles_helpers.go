package semanticcheck

import (
	"slices"

	"github.com/jacoelho/xsd/internal/types"
)

func normalizePointlessParticle(p types.Particle) types.Particle {
	for {
		mg, ok := p.(*types.ModelGroup)
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

func derivationChildren(mg *types.ModelGroup) []types.Particle {
	if mg == nil {
		return nil
	}
	children := make([]types.Particle, 0, len(mg.Particles))
	for _, child := range mg.Particles {
		children = append(children, gatherPointlessChildren(mg.Kind, child)...)
	}
	return children
}

func gatherPointlessChildren(parentKind types.GroupKind, particle types.Particle) []types.Particle {
	switch p := particle.(type) {
	case *types.ElementDecl, *types.AnyElement:
		return []types.Particle{p}
	case *types.ModelGroup:
		if !p.MinOccurs.IsOne() || !p.MaxOccurs.IsOne() {
			return []types.Particle{p}
		}
		if len(p.Particles) == 1 {
			return gatherPointlessChildren(parentKind, p.Particles[0])
		}
		if p.Kind == parentKind {
			var out []types.Particle
			for _, child := range p.Particles {
				out = append(out, gatherPointlessChildren(parentKind, child)...)
			}
			return out
		}
		return []types.Particle{p}
	default:
		return []types.Particle{p}
	}
}

// isBlockSuperset checks if restrictionBlock is a superset of baseBlock.
// Restriction block must contain all derivation methods in base block
// (i.e., restriction cannot allow more than base).
func isBlockSuperset(restrictionBlock, baseBlock types.DerivationSet) bool {
	if baseBlock.Has(types.DerivationExtension) && !restrictionBlock.Has(types.DerivationExtension) {
		return false
	}
	if baseBlock.Has(types.DerivationRestriction) && !restrictionBlock.Has(types.DerivationRestriction) {
		return false
	}
	if baseBlock.Has(types.DerivationSubstitution) && !restrictionBlock.Has(types.DerivationSubstitution) {
		return false
	}
	return true
}

// calculateEffectiveOccurrence calculates the effective minOccurs and maxOccurs
// for a model group by considering the group's occurrence and its children.
// For sequences: effective = group.occ * sum(children.occ)
// For choices: effective = group.occ * max(children.occ) for max, group.occ * min(children.minOcc) for min
func calculateEffectiveOccurrence(mg *types.ModelGroup) (minOcc, maxOcc types.Occurs) {
	groupMinOcc := mg.MinOcc()
	groupMaxOcc := mg.MaxOcc()

	if len(mg.Particles) == 0 {
		return types.OccursFromInt(0), types.OccursFromInt(0)
	}

	switch mg.Kind {
	case types.Sequence:
		sumMinOcc := types.OccursFromInt(0)
		sumMaxOcc := types.OccursFromInt(0)
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			sumMinOcc = types.AddOccurs(sumMinOcc, childMin)
			sumMaxOcc = types.AddOccurs(sumMaxOcc, childMax)
		}
		minOcc = types.MulOccurs(groupMinOcc, sumMinOcc)
		maxOcc = types.MulOccurs(groupMaxOcc, sumMaxOcc)
	case types.Choice:
		childMinOcc := types.OccursFromInt(0)
		childMaxOcc := types.OccursFromInt(0)
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
			childMaxOcc = types.MaxOccurs(childMaxOcc, childMax)
		}
		if !childMinOccSet {
			childMinOcc = types.OccursFromInt(0)
		}
		minOcc = types.MulOccurs(groupMinOcc, childMinOcc)
		maxOcc = types.MulOccurs(groupMaxOcc, childMaxOcc)
	case types.AllGroup:
		sumMinOcc := types.OccursFromInt(0)
		sumMaxOcc := types.OccursFromInt(0)
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			sumMinOcc = types.AddOccurs(sumMinOcc, childMin)
			sumMaxOcc = types.AddOccurs(sumMaxOcc, childMax)
		}
		minOcc = types.MulOccurs(groupMinOcc, sumMinOcc)
		maxOcc = types.MulOccurs(groupMaxOcc, sumMaxOcc)
	default:
		minOcc = groupMinOcc
		maxOcc = groupMaxOcc
	}
	return
}

// getParticleEffectiveOccurrence gets the effective occurrence of a single particle
func getParticleEffectiveOccurrence(p types.Particle) (minOcc, maxOcc types.Occurs) {
	switch particle := p.(type) {
	case *types.ModelGroup:
		return calculateEffectiveOccurrence(particle)
	case *types.ElementDecl:
		return particle.MinOcc(), particle.MaxOcc()
	case *types.AnyElement:
		return particle.MinOccurs, particle.MaxOccurs
	default:
		return p.MinOcc(), p.MaxOcc()
	}
}

// isEffectivelyOptional checks if a ModelGroup is effectively optional
// (all its particles are optional, making the group itself effectively optional)
func isEffectivelyOptional(mg *types.ModelGroup) bool {
	if len(mg.Particles) == 0 {
		return true
	}
	for _, particle := range mg.Particles {
		if particle.MinOcc().CmpInt(0) > 0 {
			return false
		}
		if nestedMG, ok := particle.(*types.ModelGroup); ok {
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
func isEmptiableParticle(p types.Particle) bool {
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
	case *types.ModelGroup:
		switch pt.Kind {
		case types.Sequence, types.AllGroup:
			for _, child := range pt.Particles {
				if !isEmptiableParticle(child) {
					return false
				}
			}
			return true
		case types.Choice:
			return slices.ContainsFunc(pt.Particles, isEmptiableParticle)
		default:
			return false
		}
	case *types.ElementDecl, *types.AnyElement:
		return false
	default:
		return p.MinOcc().IsZero()
	}
}
