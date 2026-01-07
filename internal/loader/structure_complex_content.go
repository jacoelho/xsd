package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateComplexContentStructure validates structural constraints of complex content
func validateComplexContentStructure(schema *schema.Schema, cc *types.ComplexContent) error {
	if cc.Extension != nil {
		if baseType, ok := schema.TypeDefs[cc.Extension.Base]; ok {
			if _, isSimple := baseType.(*types.SimpleType); isSimple {
				return fmt.Errorf("complexContent extension cannot derive from simpleType '%s'", cc.Extension.Base)
			}
		}
		if cc.Extension.Particle != nil {
			// cannot add particles when extending a simpleContent type
			baseType, ok := schema.TypeDefs[cc.Extension.Base]
			if ok {
				if baseCT, ok := baseType.(*types.ComplexType); ok {
					if _, isSimpleContent := baseCT.Content().(*types.SimpleContent); isSimpleContent {
						return fmt.Errorf("cannot extend simpleContent type '%s' with particles", cc.Extension.Base)
					}
					// XSD 1.0: Cannot extend a type with xs:all content model
					// per Errata E1-21: Can extend if base xs:all is emptiable
					if baseParticle := effectiveContentParticle(schema, baseCT); baseParticle != nil {
						if baseMG, ok := baseParticle.(*types.ModelGroup); ok && baseMG.Kind == types.AllGroup {
							if !isEmptiableParticle(baseMG) {
								return fmt.Errorf("cannot extend type with non-emptiable xs:all content model (XSD 1.0)")
							}
						}
					}
				}
			}
			// xs:all in complex content extensions
			// per XSD 1.0 Errata E1-21: xs:all can be used in extensions if base content is emptiable
			// check if extension particle is or contains xs:all (may be in a group reference)
			containsAll := false
			if mg, ok := cc.Extension.Particle.(*types.ModelGroup); ok {
				if mg.Kind == types.AllGroup {
					containsAll = true
				} else if mg.Kind == types.Sequence || mg.Kind == types.Choice {
					// check if any particle in the group is xs:all
					for _, p := range mg.Particles {
						if pmg, ok := p.(*types.ModelGroup); ok && pmg.Kind == types.AllGroup {
							containsAll = true
							break
						}
					}
				}
			}

			if containsAll {
				// check if base type's content is emptiable
				// per XSD 1.0 Errata E1-21: emptiable means minOccurs=0 or no content or empty content
				baseIsEmptiable := false
				if baseType, ok := schema.TypeDefs[cc.Extension.Base]; ok {
					// cannot extend simpleType with particles
					if _, isSimpleType := baseType.(*types.SimpleType); isSimpleType {
						return fmt.Errorf("cannot extend simpleType with complex content containing xs:all")
					}
					if baseCT, ok := baseType.(*types.ComplexType); ok {
						if baseParticle := effectiveContentParticle(schema, baseCT); baseParticle != nil {
							baseIsEmptiable = isEmptiableParticle(baseParticle)
						} else {
							baseIsEmptiable = true
						}
					}
				} else {
					// base type not found in schema - might be builtin
					// builtin types cannot have emptiable complex content
					baseIsEmptiable = false
				}
				if !baseIsEmptiable {
					return fmt.Errorf("xs:all cannot be used in complex content extensions unless base content is emptiable (XSD 1.0 Errata E1-21)")
				}
			}
			if err := validateParticleStructure(schema, cc.Extension.Particle, nil); err != nil {
				return err
			}
			if err := validateElementDeclarationsConsistentInParticle(schema, cc.Extension.Particle); err != nil {
				return err
			}
		}
	}
	if cc.Restriction != nil {
		if baseType, ok := schema.TypeDefs[cc.Restriction.Base]; ok {
			if _, isSimple := baseType.(*types.SimpleType); isSimple {
				return fmt.Errorf("complexContent restriction cannot derive from simpleType '%s'", cc.Restriction.Base)
			}
			if baseCT, ok := baseType.(*types.ComplexType); ok {
				if _, isSimpleContent := baseCT.Content().(*types.SimpleContent); isSimpleContent {
					return fmt.Errorf("complexContent restriction cannot derive from simpleContent type '%s'", cc.Restriction.Base)
				}
			}
		}
		if cc.Restriction.Particle != nil {
			if baseType, ok := schema.TypeDefs[cc.Restriction.Base]; ok {
				if baseParticle := effectiveContentParticle(schema, baseType); baseParticle != nil {
					if err := validateParticlePairRestriction(schema, baseParticle, cc.Restriction.Particle); err != nil {
						return err
					}
				}
			}
			if err := validateParticleStructure(schema, cc.Restriction.Particle, nil); err != nil {
				return err
			}
			if err := validateElementDeclarationsConsistentInParticle(schema, cc.Restriction.Particle); err != nil {
				return err
			}
		}
		// validate that attributes in restriction match base type's attributes (only use can differ)
		// per XSD spec (cos-ct-derived-ok): in complexContent restriction, attributes must have the same type as base attributes
		baseType, ok := schema.TypeDefs[cc.Restriction.Base]
		if ok {
			if baseCT, ok := baseType.(*types.ComplexType); ok {
				restrictionAttrs := append([]*types.AttributeDecl(nil), cc.Restriction.Attributes...)
				restrictionAttrs = append(restrictionAttrs, collectAttributesFromGroups(schema, cc.Restriction.AttrGroups, nil)...)
				if err := validateRestrictionAttributes(schema, baseCT, restrictionAttrs, "complexContent restriction"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}