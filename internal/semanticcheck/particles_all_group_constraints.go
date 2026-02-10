package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

func validateAllGroupConstraints(group *model.ModelGroup, parentKind *model.GroupKind) error {
	if parentKind != nil {
		if *parentKind == model.Sequence || *parentKind == model.Choice {
			return fmt.Errorf("xs:all cannot appear as a child of xs:sequence or xs:choice (XSD 1.0)")
		}
	}
	if err := validateAllGroupUniqueElements(group.Particles); err != nil {
		return err
	}
	if err := validateAllGroupOccurrence(group); err != nil {
		return err
	}
	if err := validateAllGroupParticleOccurs(group.Particles); err != nil {
		return err
	}
	if err := validateAllGroupNested(group.Particles); err != nil {
		return err
	}
	return nil
}

func validateAllGroupUniqueElements(particles []model.Particle) error {
	seenElements := make(map[model.QName]bool)
	for _, childParticle := range particles {
		childElem, ok := childParticle.(*model.ElementDecl)
		if !ok {
			continue
		}
		if seenElements[childElem.Name] {
			return fmt.Errorf("xs:all: duplicate element declaration '%s'", childElem.Name)
		}
		seenElements[childElem.Name] = true
	}
	return nil
}

func validateAllGroupOccurrence(group *model.ModelGroup) error {
	if !group.MinOccurs.IsZero() && !group.MinOccurs.IsOne() {
		return fmt.Errorf("xs:all must have minOccurs='0' or '1' (got %s)", group.MinOccurs)
	}
	if !group.MaxOccurs.IsOne() {
		return fmt.Errorf("xs:all must have maxOccurs='1' (got %s)", group.MaxOccurs)
	}
	return nil
}

func validateAllGroupParticleOccurs(particles []model.Particle) error {
	for _, childParticle := range particles {
		if childParticle.MaxOcc().CmpInt(1) > 0 {
			return fmt.Errorf("xs:all: all particles must have maxOccurs <= 1 (got %s)", childParticle.MaxOcc())
		}
	}
	return nil
}

func validateAllGroupNested(particles []model.Particle) error {
	for _, childParticle := range particles {
		childMG, ok := childParticle.(*model.ModelGroup)
		if !ok {
			continue
		}
		if childMG.Kind == model.AllGroup && childMG.MinOccurs.CmpInt(0) > 0 {
			return fmt.Errorf("xs:all: nested xs:all cannot have minOccurs > 0 (got %s)", childMG.MinOccurs)
		}
	}
	return nil
}
