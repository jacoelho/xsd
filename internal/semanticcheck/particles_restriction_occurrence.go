package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

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
	return nil
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
