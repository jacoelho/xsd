package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
)

func validateElementPairRestriction(schema *parser.Schema, baseParticle, restrictionParticle model.Particle) (bool, error) {
	baseElem, baseIsElem := baseParticle.(*model.ElementDecl)
	if !baseIsElem {
		return false, nil
	}
	switch restriction := restrictionParticle.(type) {
	case *model.ElementDecl:
		return true, validateElementToElementRestriction(schema, baseElem, restriction)
	case *model.ModelGroup:
		return true, validateElementToChoiceRestriction(schema, baseElem, restriction)
	default:
		return false, nil
	}
}

func validateElementToElementRestriction(schema *parser.Schema, baseElem, restrictionElem *model.ElementDecl) error {
	if restrictionElem.MinOcc().IsZero() && restrictionElem.MaxOcc().IsZero() {
		return nil
	}
	if baseElem.Name != restrictionElem.Name {
		if !isSubstitutableElement(schema, baseElem.Name, restrictionElem.Name) {
			return fmt.Errorf("ComplexContent restriction: element name mismatch (%s vs %s)", baseElem.Name, restrictionElem.Name)
		}
	}
	return validateElementRestriction(schema, baseElem, restrictionElem)
}

func validateElementToChoiceRestriction(schema *parser.Schema, baseElem *model.ElementDecl, restrictionGroup *model.ModelGroup) error {
	if restrictionGroup.Kind != model.Choice {
		return fmt.Errorf("ComplexContent restriction: cannot restrict element %s to model group", baseElem.Name)
	}
	for _, p := range restrictionGroup.Particles {
		if p.MinOcc().IsZero() && p.MaxOcc().IsZero() {
			continue
		}
		childElem, ok := p.(*model.ElementDecl)
		if !ok {
			return fmt.Errorf("ComplexContent restriction: element %s restriction choice must contain only elements", baseElem.Name)
		}
		if err := validateParticlePairRestriction(schema, baseElem, childElem); err != nil {
			return fmt.Errorf("ComplexContent restriction: element %s restriction choice contains invalid particle: %w", baseElem.Name, err)
		}
	}
	return nil
}
