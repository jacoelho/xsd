package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateParticleRestrictionWithKindChange(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup) error {
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
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

	if baseMG.Kind == model.Choice && restrictionMG.Kind == model.Sequence {
		return validateChoiceToSequenceRestriction(schema, baseMG, restrictionMG, baseChildren, restrictionChildren)
	}

	return fmt.Errorf("ComplexContent restriction: invalid model group kind change from %s to %s", groupKindName(baseMG.Kind), groupKindName(restrictionMG.Kind))
}

func validateSequenceToChoiceRestriction(baseMG, restrictionMG *model.ModelGroup) (bool, error) {
	if baseMG.Kind != model.Sequence || restrictionMG.Kind != model.Choice {
		return false, nil
	}
	return true, fmt.Errorf("ComplexContent restriction: cannot restrict sequence to choice")
}

func validateKindChangeWithWildcard(schema *parser.Schema, baseChildren []model.Particle, restrictionMG *model.ModelGroup, restrictionChildren []model.Particle) error {
	for _, baseParticle := range baseChildren {
		if baseWildcard, isWildcard := baseParticle.(*model.AnyElement); isWildcard {
			if err := validateParticlePairRestriction(schema, baseWildcard, restrictionMG); err == nil {
				return nil
			}
		}
	}
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

func validateAllGroupKindChange(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup, baseChildren, restrictionChildren []model.Particle) (bool, error) {
	if restrictionMG.Kind == model.AllGroup && baseMG.Kind != model.AllGroup {
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

	if baseMG.Kind == model.AllGroup && restrictionMG.Kind != model.AllGroup {
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

func validateChoiceToSequenceRestriction(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup, baseChildren, restrictionChildren []model.Particle) error {
	derivedCount := len(restrictionChildren)
	countOccurs := model.OccursFromInt(derivedCount)
	derivedMin := model.MulOccurs(restrictionMG.MinOccurs, countOccurs)
	derivedMax := model.MulOccurs(restrictionMG.MaxOccurs, countOccurs)
	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), derivedMin, derivedMax); err != nil {
		return err
	}
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
