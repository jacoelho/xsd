package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateParticleRestriction validates that particles in a restriction are valid restrictions of base particles
func validateParticleRestriction(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup) error {
	if baseMG.MaxOcc().IsZero() && restrictionMG.MaxOcc().IsZero() {
		return nil
	}
	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), restrictionMG.MinOcc(), restrictionMG.MaxOcc()); err != nil {
		return err
	}
	if err := validateSingleWildcardGroupRestriction(schema, baseMG, restrictionMG); err != nil {
		return err
	}
	if baseMG.Kind != restrictionMG.Kind {
		return validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
	}
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
	switch baseMG.Kind {
	case model.Sequence:
		return validateSequenceRestriction(schema, baseChildren, restrictionChildren)
	case model.Choice:
		return validateChoiceRestriction(schema, baseChildren, restrictionChildren)
	case model.AllGroup:
		return validateAllGroupRestriction(schema, baseMG, restrictionMG)
	}
	return nil
}

func validateSingleWildcardGroupRestriction(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup) error {
	if len(baseMG.Particles) != 1 {
		return nil
	}
	baseAny, ok := baseMG.Particles[0].(*model.AnyElement)
	if !ok {
		return nil
	}
	return validateParticlePairRestriction(schema, baseAny, restrictionMG)
}

func validateSequenceRestriction(schema *parser.Schema, baseChildren, restrictionChildren []model.Particle) error {
	baseIdx := 0
	matchedBaseParticles := make(map[int]bool)
	for _, restrictionParticle := range restrictionChildren {
		if restrictionParticle.MaxOcc().IsZero() && restrictionParticle.MinOcc().IsZero() {
			continue
		}
		found := false
		for baseIdx < len(baseChildren) {
			baseParticle := baseChildren[baseIdx]
			err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle)
			if err == nil {
				matchedBaseParticles[baseIdx] = true
				if baseAny, isWildcard := baseParticle.(*model.AnyElement); isWildcard {
					if baseAny.MaxOccurs.IsOne() {
						baseIdx++
					}
				} else {
					baseIdx++
				}
				found = true
				break
			}
			skippable := baseParticle.MinOcc().IsZero()
			if !skippable {
				if baseGroup, ok := baseParticle.(*model.ModelGroup); ok {
					skippable = isEffectivelyOptional(baseGroup)
				}
			}
			if skippable {
				baseIdx++
				continue
			}
			return err
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle")
		}
	}
	for i := baseIdx; i < len(baseChildren); i++ {
		baseParticle := baseChildren[i]
		if matchedBaseParticles[i] {
			continue
		}
		if baseParticle.MinOcc().CmpInt(0) > 0 {
			if baseMG2, ok := baseParticle.(*model.ModelGroup); ok {
				if isEffectivelyOptional(baseMG2) {
					continue
				}
			}
			return fmt.Errorf("ComplexContent restriction: required particle at position %d is missing", i)
		}
	}
	return nil
}

func validateChoiceRestriction(schema *parser.Schema, baseChildren, restrictionChildren []model.Particle) error {
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

func validateAllGroupRestriction(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup) error {
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
				if baseGroup, ok := baseParticle.(*model.ModelGroup); ok {
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
			if baseGroup, ok := baseParticle.(*model.ModelGroup); ok {
				if isEffectivelyOptional(baseGroup) {
					continue
				}
			}
			return fmt.Errorf("ComplexContent restriction: required particle at position %d is missing", i)
		}
	}
	return nil
}
