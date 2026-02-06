package semanticcheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func validateParticleRestrictionWithKindChange(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
	// special case: if base contains a wildcard, the restriction can use any compositor
	// per XSD spec section 7.2.5 (NSRecurseCheckCardinality), a model group can restrict a wildcard
	// with any compositor as long as the model group's particles are valid restrictions of the wildcard
	baseHasWildcard := modelGroupContainsWildcard(baseMG)

	if baseHasWildcard {
		return validateKindChangeWithWildcard(schema, baseChildren, restrictionMG, restrictionChildren)
	}

	if handled, err := validateAllGroupKindChange(schema, baseMG, restrictionMG, baseChildren, restrictionChildren); handled {
		return err
	}

	if handled, err := validateSequenceToChoiceRestriction(baseMG, restrictionMG); handled {
		return err
	}

	// choice -> sequence: Valid if all sequence particles match some choice particle
	if baseMG.Kind == types.Choice && restrictionMG.Kind == types.Sequence {
		return validateChoiceToSequenceRestriction(schema, baseMG, restrictionMG, baseChildren, restrictionChildren)
	}

	// other kind changes should not reach here due to early returns above
	return fmt.Errorf("ComplexContent restriction: invalid model group kind change from %s to %s", groupKindName(baseMG.Kind), groupKindName(restrictionMG.Kind))
}

func validateSequenceToChoiceRestriction(baseMG, restrictionMG *types.ModelGroup) (bool, error) {
	if baseMG.Kind != types.Sequence || restrictionMG.Kind != types.Choice {
		return false, nil
	}
	return true, fmt.Errorf("ComplexContent restriction: cannot restrict sequence to choice")
}

func validateKindChangeWithWildcard(schema *parser.Schema, baseChildren []types.Particle, restrictionMG *types.ModelGroup, restrictionChildren []types.Particle) error {
	// find the wildcard in the base and validate the entire restriction group against it
	for _, baseParticle := range baseChildren {
		if baseWildcard, isWildcard := baseParticle.(*types.AnyElement); isWildcard {
			// this uses the ModelGroup:Wildcard derivation rule (NSRecurseCheckCardinality)
			if err := validateParticlePairRestriction(schema, baseWildcard, restrictionMG); err == nil {
				return nil
			}
		}
	}
	// if we found wildcards but couldn't validate against them, try particle-by-particle validation
	for _, restrictionParticle := range restrictionChildren {
		found := false
		for _, baseParticle := range baseChildren {
			if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
		}
	}
	return nil
}

func validateAllGroupKindChange(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup, baseChildren, restrictionChildren []types.Particle) (bool, error) {
	// no wildcards - apply strict compositor change rules
	// xs:all has unique semantics - non-all cannot restrict to xs:all
	// exception: xs:all with a single element can restrict sequence/choice
	// (no ordering ambiguity with one element)
	if restrictionMG.Kind == types.AllGroup && baseMG.Kind != types.AllGroup {
		// allow xs:all with single element to restrict sequence/choice
		if len(restrictionChildren) == 1 {
			restrictionParticle := restrictionChildren[0]
			for _, baseParticle := range baseChildren {
				if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
					return true, nil
				}
			}
			return true, fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
		}
		return true, fmt.Errorf("ComplexContent restriction: cannot restrict %s to xs:all", groupKindName(baseMG.Kind))
	}

	// xs:all -> sequence/choice: Valid if restriction particles match base particles
	// per XSD spec, restricting xs:all to sequence/choice adds ordering constraints
	if baseMG.Kind == types.AllGroup && restrictionMG.Kind != types.AllGroup {
		for _, restrictionParticle := range restrictionChildren {
			found := false
			for _, baseParticle := range baseChildren {
				if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
					found = true
					break
				}
			}
			if !found {
				return true, fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
			}
		}
		return true, nil
	}

	return false, nil
}

func validateChoiceToSequenceRestriction(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup, baseChildren, restrictionChildren []types.Particle) error {
	derivedCount := len(restrictionChildren)
	countOccurs := types.OccursFromInt(derivedCount)
	derivedMin := types.MulOccurs(restrictionMG.MinOccurs, countOccurs)
	derivedMax := types.MulOccurs(restrictionMG.MaxOccurs, countOccurs)
	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), derivedMin, derivedMax); err != nil {
		return err
	}
	// each restriction particle must be a valid restriction of at least one base particle
	for _, restrictionParticle := range restrictionChildren {
		found := false
		for _, baseParticle := range baseChildren {
			if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle in choice")
		}
	}
	return nil
}

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
		// for sequences, sum all children's occurrences
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
		// for choices, take the min of children's minOccurs (since only one branch is taken)
		// and max of children's maxOccurs
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
		// for all groups, sum all children (like sequence, all must appear)
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
		// recursively check nested model groups
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
	// maxOccurs=0 means the particle contributes nothing.
	if p.MaxOcc().IsZero() {
		return true
	}
	// minOccurs=0 means we can choose zero occurrences.
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
