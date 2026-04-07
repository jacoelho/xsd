package semanticcheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

// validateComplexContentStructure validates structural constraints of complex content
func validateComplexContentStructure(schema *parser.Schema, cc *model.ComplexContent) error {
	if cc.Extension != nil {
		if err := validateComplexContentExtension(schema, cc.Extension); err != nil {
			return err
		}
	}
	if cc.Restriction != nil {
		if err := validateComplexContentRestriction(schema, cc.Restriction); err != nil {
			return err
		}
	}
	return nil
}

func validateComplexContentExtension(schema *parser.Schema, ext *model.Extension) error {
	baseType, baseOK := semantics.LookupType(schema, ext.Base)
	if baseOK {
		if _, isSimple := baseType.(*model.SimpleType); isSimple {
			return fmt.Errorf("complexContent extension cannot derive from simpleType '%s'", ext.Base)
		}
	}
	if ext.Particle == nil {
		return nil
	}
	baseParticle, err := validateComplexExtensionBase(schema, ext.Base, baseType)
	if err != nil {
		return err
	}
	if extensionContainsAll(ext.Particle) && !baseParticleIsEmptiable(schema, baseType, baseOK, baseParticle) {
		return fmt.Errorf("xs:all cannot be used in complex content extensions unless base content is emptiable (XSD 1.0 Errata E1-21)")
	}
	return validateComplexContentParticle(schema, ext.Particle)
}

func validateComplexExtensionBase(schema *parser.Schema, baseQName model.QName, baseType model.Type) (model.Particle, error) {
	baseCT, ok := baseType.(*model.ComplexType)
	if !ok {
		return nil, nil
	}
	if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); isSimpleContent {
		return nil, fmt.Errorf("cannot extend simpleContent type '%s' with particles", baseQName)
	}
	baseParticle := semantics.EffectiveContentParticle(schema, baseCT)
	if baseMG, ok := baseParticle.(*model.ModelGroup); ok && baseMG.Kind == model.AllGroup && !isEmptiableParticle(baseMG) {
		return nil, fmt.Errorf("cannot extend type with non-emptiable xs:all content model (XSD 1.0)")
	}
	return baseParticle, nil
}

func extensionContainsAll(particle model.Particle) bool {
	mg, ok := particle.(*model.ModelGroup)
	if !ok {
		return false
	}
	if mg.Kind == model.AllGroup {
		return true
	}
	if mg.Kind != model.Sequence && mg.Kind != model.Choice {
		return false
	}
	for _, p := range mg.Particles {
		pmg, ok := p.(*model.ModelGroup)
		if ok && pmg.Kind == model.AllGroup {
			return true
		}
	}
	return false
}

func baseParticleIsEmptiable(schema *parser.Schema, baseType model.Type, baseOK bool, baseParticle model.Particle) bool {
	if !baseOK {
		return false
	}
	if baseParticle != nil {
		return isEmptiableParticle(baseParticle)
	}
	baseCT, ok := baseType.(*model.ComplexType)
	if !ok {
		return false
	}
	baseParticle = semantics.EffectiveContentParticle(schema, baseCT)
	if baseParticle == nil {
		return true
	}
	return isEmptiableParticle(baseParticle)
}

func validateComplexContentRestriction(schema *parser.Schema, restriction *model.Restriction) error {
	baseType, baseOK := semantics.LookupType(schema, restriction.Base)
	if err := validateComplexRestrictionBase(restriction.Base, baseType, baseOK); err != nil {
		return err
	}
	if err := validateComplexRestrictionParticle(schema, baseType, baseOK, restriction.Particle); err != nil {
		return err
	}
	if baseCT, ok := baseType.(*model.ComplexType); ok {
		return validateComplexRestrictionAttributes(schema, baseCT, restriction)
	}
	return nil
}

func validateComplexRestrictionBase(baseQName model.QName, baseType model.Type, baseOK bool) error {
	if !baseOK {
		return nil
	}
	if _, isSimple := baseType.(*model.SimpleType); isSimple {
		return fmt.Errorf("complexContent restriction cannot derive from simpleType '%s'", baseQName)
	}
	if baseCT, ok := baseType.(*model.ComplexType); ok {
		if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); isSimpleContent {
			return fmt.Errorf("complexContent restriction cannot derive from simpleContent type '%s'", baseQName)
		}
	}
	return nil
}

func validateComplexRestrictionParticle(schema *parser.Schema, baseType model.Type, baseOK bool, particle model.Particle) error {
	if particle == nil {
		return nil
	}
	if baseOK {
		if baseParticle := semantics.EffectiveContentParticle(schema, baseType); baseParticle != nil {
			if err := validateParticlePairRestriction(schema, baseParticle, particle); err != nil {
				return err
			}
		}
	}
	return validateComplexContentParticle(schema, particle)
}

func validateComplexContentParticle(schema *parser.Schema, particle model.Particle) error {
	if err := validateParticleStructure(schema, particle); err != nil {
		return err
	}
	return validateElementDeclarationsConsistentInParticle(schema, particle)
}

func validateComplexRestrictionAttributes(schema *parser.Schema, baseCT *model.ComplexType, restriction *model.Restriction) error {
	restrictionAttrs := slices.Clone(restriction.Attributes)
	restrictionAttrs = append(restrictionAttrs, collectAttributesFromGroups(schema, restriction.AttrGroups)...)
	return validateRestrictionAttributes(schema, baseCT, restrictionAttrs, "complexContent restriction")
}
