package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
)

// validateParticlePairRestriction validates that a restriction particle is a valid restriction of a base particle
func validateParticlePairRestriction(schema *parser.Schema, baseParticle, restrictionParticle model.Particle) error {
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

func validateModelGroupElementRestriction(schema *parser.Schema, baseParticle, restrictionParticle model.Particle) (bool, error) {
	baseMG, baseIsMG := baseParticle.(*model.ModelGroup)
	restrictionElem, restrictionIsElem := restrictionParticle.(*model.ElementDecl)
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

func validateModelGroupPairRestriction(schema *parser.Schema, baseParticle, restrictionParticle model.Particle) (bool, error) {
	baseMG, baseIsMG := baseParticle.(*model.ModelGroup)
	restrictionMG, restrictionIsMG := restrictionParticle.(*model.ModelGroup)
	if !baseIsMG || !restrictionIsMG {
		return false, nil
	}
	if baseMG.Kind != restrictionMG.Kind {
		return true, validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
	}
	return true, validateParticleRestriction(schema, baseMG, restrictionMG)
}
