package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateParticleRestriction validates that particles in a restriction are valid restrictions of base particles
func validateParticleRestriction(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	// if both model groups have maxOccurs=0, the content never occurs, so children
	// constraints are irrelevant. Skip child validation in this case.
	if baseMG.MaxOcc().IsZero() && restrictionMG.MaxOcc().IsZero() {
		return nil
	}
	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), restrictionMG.MinOcc(), restrictionMG.MaxOcc()); err != nil {
		return err
	}
	if err := validateSingleWildcardGroupRestriction(schema, baseMG, restrictionMG); err != nil {
		return err
	}
	// handle model group kind changes: if kinds differ, validate that restriction is valid
	if baseMG.Kind != restrictionMG.Kind {
		// allow kind changes if restriction particles are valid restrictions of base particles
		// for example: choice -> sequence is valid if sequence elements match choice elements
		// for example: sequence with wildcard -> all with elements is valid
		return validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
	}
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
	// same kind - validate normally
	switch baseMG.Kind {
	case types.Sequence:
		return validateSequenceRestriction(schema, baseChildren, restrictionChildren)
	case types.Choice:
		return validateChoiceRestriction(schema, baseChildren, restrictionChildren)
	case types.AllGroup:
		return validateAllGroupRestriction(schema, baseMG, restrictionMG)
	}
	return nil
}

func validateSingleWildcardGroupRestriction(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	// if the base group is a single wildcard particle, validate the restriction group
	// directly against the wildcard using NSRecurseCheckCardinality semantics.
	if len(baseMG.Particles) != 1 {
		return nil
	}
	baseAny, ok := baseMG.Particles[0].(*types.AnyElement)
	if !ok {
		return nil
	}
	return validateParticlePairRestriction(schema, baseAny, restrictionMG)
}

func validateSequenceRestriction(schema *parser.Schema, baseChildren, restrictionChildren []types.Particle) error {
	// for sequence, particles must match in order
	// optional particles (minOccurs=0) can be removed, but required particles must be present
	baseIdx := 0
	matchedBaseParticles := make(map[int]bool) // track which base particles have been matched
	for _, restrictionParticle := range restrictionChildren {
		if restrictionParticle.MaxOcc().IsZero() && restrictionParticle.MinOcc().IsZero() {
			continue
		}
		found := false
		for baseIdx < len(baseChildren) {
			baseParticle := baseChildren[baseIdx]
			// try to match this restriction particle with the current base particle
			err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle)
			if err == nil {
				// match found - mark this base particle as matched
				matchedBaseParticles[baseIdx] = true
				// if base particle is a wildcard with maxOccurs > 1 or unbounded, we can match multiple restriction particles to it
				// otherwise, both particles advance
				if baseAny, isWildcard := baseParticle.(*types.AnyElement); isWildcard {
					// wildcard can match multiple restriction particles
					// only advance past wildcard if maxOccurs=1
					if baseAny.MaxOccurs.IsOne() {
						baseIdx++ // advance past wildcard with maxOccurs=1
					}
					// if maxOccurs > 1 or unbounded, stay on wildcard (don't increment baseIdx)
				} else {
					baseIdx++ // advance past non-wildcard
				}
				found = true
				break
			}
			skippable := baseParticle.MinOcc().IsZero()
			if !skippable {
				if baseGroup, ok := baseParticle.(*types.ModelGroup); ok {
					skippable = isEffectivelyOptional(baseGroup)
				}
			}
			if skippable {
				baseIdx++
				continue
			}
			// required particle cannot be skipped - return the original error
			return err
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle")
		}
	}
	// check if any remaining required base particles were skipped
	for i := baseIdx; i < len(baseChildren); i++ {
		baseParticle := baseChildren[i]
		// skip if this particle was already matched
		if matchedBaseParticles[i] {
			continue
		}
		if baseParticle.MinOcc().CmpInt(0) > 0 {
			// check if it's effectively optional (contains only optional content)
			if baseMG2, ok := baseParticle.(*types.ModelGroup); ok {
				if isEffectivelyOptional(baseMG2) {
					// effectively optional - can be skipped
					continue
				}
			}
			return fmt.Errorf("ComplexContent restriction: required particle at position %d is missing", i)
		}
	}
	return nil
}

func validateChoiceRestriction(schema *parser.Schema, baseChildren, restrictionChildren []types.Particle) error {
	// choice uses RecurseLax: match restriction particles to base particles in order.
	baseIdx := 0
	for _, restrictionParticle := range restrictionChildren {
		if restrictionParticle.MaxOcc().IsZero() && restrictionParticle.MinOcc().IsZero() {
			continue
		}
		found := false
		for baseIdx < len(baseChildren) {
			baseParticle := baseChildren[baseIdx]
			baseIdx++
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

func validateAllGroupRestriction(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)

	baseIdx := 0
	for _, restrictionParticle := range restrictionChildren {
		if restrictionParticle.MaxOcc().IsZero() && restrictionParticle.MinOcc().IsZero() {
			continue
		}
		found := false
		for baseIdx < len(baseChildren) {
			baseParticle := baseChildren[baseIdx]
			baseIdx++
			err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle)
			if err == nil {
				found = true
				break
			}
			skippable := baseParticle.MinOcc().IsZero()
			if !skippable {
				if baseGroup, ok := baseParticle.(*types.ModelGroup); ok {
					skippable = isEffectivelyOptional(baseGroup)
				}
			}
			if !skippable {
				return err
			}
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle in all group")
		}
	}
	for i := baseIdx; i < len(baseChildren); i++ {
		baseParticle := baseChildren[i]
		if baseParticle.MinOcc().CmpInt(0) > 0 {
			if baseGroup, ok := baseParticle.(*types.ModelGroup); ok {
				if isEffectivelyOptional(baseGroup) {
					continue
				}
			}
			return fmt.Errorf("ComplexContent restriction: required particle at position %d is missing", i)
		}
	}
	return nil
}

// validateOccurrenceConstraints validates occurrence constraints for particle restrictions
// In a restriction:
// - minOccurs must be >= base minOccurs (can require more)
// - maxOccurs must be <= base maxOccurs (can allow fewer)
// - minOccurs must be <= base maxOccurs (can't require more than base allows)
func validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc types.Occurs) error {
	if baseMinOcc.IsOverflow() || baseMaxOcc.IsOverflow() || restrictionMinOcc.IsOverflow() || restrictionMaxOcc.IsOverflow() {
		return fmt.Errorf("%w: occurrence value exceeds uint32", types.ErrOccursOverflow)
	}
	if restrictionMinOcc.Cmp(baseMinOcc) < 0 {
		return fmt.Errorf("ComplexContent restriction: minOccurs (%s) must be >= base minOccurs (%s)", restrictionMinOcc, baseMinOcc)
	}
	if !baseMaxOcc.IsUnbounded() {
		if restrictionMaxOcc.IsUnbounded() {
			return fmt.Errorf("ComplexContent restriction: maxOccurs cannot be unbounded when base maxOccurs is bounded (%s)", baseMaxOcc)
		}
		if restrictionMaxOcc.Cmp(baseMaxOcc) > 0 {
			return fmt.Errorf("ComplexContent restriction: maxOccurs (%s) must be <= base maxOccurs (%s)", restrictionMaxOcc, baseMaxOcc)
		}
		if restrictionMinOcc.Cmp(baseMaxOcc) > 0 {
			return fmt.Errorf("ComplexContent restriction: minOccurs (%s) must be <= base maxOccurs (%s)", restrictionMinOcc, baseMaxOcc)
		}
	}
	// base has unbounded maxOccurs, restriction can have any minOccurs >= base minOccurs
	// both unbounded or restriction bounded with base unbounded are valid
	return nil
}

// validateParticlePairRestriction validates that a restriction particle is a valid restriction of a base particle
func validateParticlePairRestriction(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) error {
	baseParticle = normalizePointlessParticle(baseParticle)
	restrictionParticle = normalizePointlessParticle(restrictionParticle)

	if handled, err := validateWildcardBaseRestriction(schema, baseParticle, restrictionParticle); handled {
		return err
	}

	if handled, err := validateModelGroupElementRestriction(schema, baseParticle, restrictionParticle); handled {
		return err
	}

	if err := validateParticlePairOccurrence(baseParticle, restrictionParticle); err != nil {
		return err
	}

	if handled, err := validateWildcardPairRestriction(baseParticle, restrictionParticle); handled {
		return err
	}

	if handled, err := validateElementPairRestriction(schema, baseParticle, restrictionParticle); handled {
		return err
	}

	if handled, err := validateModelGroupPairRestriction(schema, baseParticle, restrictionParticle); handled {
		return err
	}

	return nil
}

func validateModelGroupElementRestriction(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) (bool, error) {
	baseMG, baseIsMG := baseParticle.(*types.ModelGroup)
	restrictionElem, restrictionIsElem := restrictionParticle.(*types.ElementDecl)
	if !baseIsMG || !restrictionIsElem {
		return false, nil
	}
	matched, err := validateModelGroupToElementRestriction(schema, baseMG, restrictionElem)
	if err != nil {
		return true, err
	}
	if matched {
		return true, nil
	}
	return true, fmt.Errorf("ComplexContent restriction: element %s does not match any element in base model group", restrictionElem.Name)
}

func effectiveParticleOccurrence(baseParticle, restrictionParticle types.Particle) (types.Occurs, types.Occurs, types.Occurs, types.Occurs) {
	baseMinOcc := baseParticle.MinOcc()
	baseMaxOcc := baseParticle.MaxOcc()
	restrictionMinOcc := restrictionParticle.MinOcc()
	restrictionMaxOcc := restrictionParticle.MaxOcc()
	if baseMG, ok := baseParticle.(*types.ModelGroup); ok {
		if restrictionMG, ok := restrictionParticle.(*types.ModelGroup); ok {
			baseMinOcc, baseMaxOcc = calculateEffectiveOccurrence(baseMG)
			restrictionMinOcc, restrictionMaxOcc = calculateEffectiveOccurrence(restrictionMG)
		}
	}
	return baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc
}

func validateParticlePairOccurrence(baseParticle, restrictionParticle types.Particle) error {
	baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc := effectiveParticleOccurrence(baseParticle, restrictionParticle)
	return validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc)
}

func validateModelGroupPairRestriction(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) (bool, error) {
	baseMG, baseIsMG := baseParticle.(*types.ModelGroup)
	restrictionMG, restrictionIsMG := restrictionParticle.(*types.ModelGroup)
	if !baseIsMG || !restrictionIsMG {
		return false, nil
	}
	if baseMG.Kind != restrictionMG.Kind {
		return true, validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
	}
	return true, validateParticleRestriction(schema, baseMG, restrictionMG)
}
