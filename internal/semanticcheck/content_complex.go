package semanticcheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typechain"
)

// validateComplexContentStructure validates structural constraints of complex content
func validateComplexContentStructure(schema *parser.Schema, cc *model.ComplexContent) error {
	if cc.Extension != nil {
		baseType, baseOK := typechain.LookupType(schema, cc.Extension.Base)
		if baseOK {
			if _, isSimple := baseType.(*model.SimpleType); isSimple {
				return fmt.Errorf("complexContent extension cannot derive from simpleType '%s'", cc.Extension.Base)
			}
		}
		if cc.Extension.Particle != nil {
			var baseParticle model.Particle
			if baseCT, ok := baseType.(*model.ComplexType); ok {
				if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); isSimpleContent {
					return fmt.Errorf("cannot extend simpleContent type '%s' with particles", cc.Extension.Base)
				}
				baseParticle = typechain.EffectiveContentParticle(schema, baseCT)
				if baseParticle != nil {
					if baseMG, ok := baseParticle.(*model.ModelGroup); ok && baseMG.Kind == model.AllGroup {
						if !isEmptiableParticle(baseMG) {
							return fmt.Errorf("cannot extend type with non-emptiable xs:all content model (XSD 1.0)")
						}
					}
				}
			}
			containsAll := false
			if mg, ok := cc.Extension.Particle.(*model.ModelGroup); ok {
				if mg.Kind == model.AllGroup {
					containsAll = true
				} else if mg.Kind == model.Sequence || mg.Kind == model.Choice {
					for _, p := range mg.Particles {
						if pmg, ok := p.(*model.ModelGroup); ok && pmg.Kind == model.AllGroup {
							containsAll = true
							break
						}
					}
				}
			}

			if containsAll {
				baseIsEmptiable := false
				if baseOK {
					if baseCT, ok := baseType.(*model.ComplexType); ok {
						if baseParticle == nil {
							baseParticle = typechain.EffectiveContentParticle(schema, baseCT)
						}
						if baseParticle != nil {
							baseIsEmptiable = isEmptiableParticle(baseParticle)
						} else {
							baseIsEmptiable = true
						}
					}
				} else {
					baseIsEmptiable = false
				}
				if !baseIsEmptiable {
					return fmt.Errorf("xs:all cannot be used in complex content extensions unless base content is emptiable (XSD 1.0 Errata E1-21)")
				}
			}
			if err := validateParticleStructure(schema, cc.Extension.Particle); err != nil {
				return err
			}
			if err := validateElementDeclarationsConsistentInParticle(schema, cc.Extension.Particle); err != nil {
				return err
			}
		}
	}
	if cc.Restriction != nil {
		baseType, baseOK := typechain.LookupType(schema, cc.Restriction.Base)
		if baseOK {
			if _, isSimple := baseType.(*model.SimpleType); isSimple {
				return fmt.Errorf("complexContent restriction cannot derive from simpleType '%s'", cc.Restriction.Base)
			}
			if baseCT, ok := baseType.(*model.ComplexType); ok {
				if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); isSimpleContent {
					return fmt.Errorf("complexContent restriction cannot derive from simpleContent type '%s'", cc.Restriction.Base)
				}
			}
		}
		if cc.Restriction.Particle != nil {
			if baseOK {
				if baseParticle := typechain.EffectiveContentParticle(schema, baseType); baseParticle != nil {
					if err := validateParticlePairRestriction(schema, baseParticle, cc.Restriction.Particle); err != nil {
						return err
					}
				}
			}
			if err := validateParticleStructure(schema, cc.Restriction.Particle); err != nil {
				return err
			}
			if err := validateElementDeclarationsConsistentInParticle(schema, cc.Restriction.Particle); err != nil {
				return err
			}
		}
		if baseCT, ok := baseType.(*model.ComplexType); ok {
			restrictionAttrs := slices.Clone(cc.Restriction.Attributes)
			restrictionAttrs = append(restrictionAttrs, collectAttributesFromGroups(schema, cc.Restriction.AttrGroups)...)
			if err := validateRestrictionAttributes(schema, baseCT, restrictionAttrs, "complexContent restriction"); err != nil {
				return err
			}
		}
	}
	return nil
}
