package schemacheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateContentStructure validates structural constraints of content
// Does not validate references (which might be forward references or imports)
// isInline indicates if this content is part of an inline complexType (local element)
func validateContentStructure(schema *parser.Schema, content types.Content, isInline bool) error {
	switch c := content.(type) {
	case *types.ElementContent:
		if err := validateParticleStructure(schema, c.Particle, nil); err != nil {
			return err
		}
		if err := validateElementDeclarationsConsistentInParticle(schema, c.Particle); err != nil {
			return err
		}
	case *types.SimpleContent:
		return validateSimpleContentStructure(schema, c, isInline)
	case *types.ComplexContent:
		return validateComplexContentStructure(schema, c)
	case *types.EmptyContent:
		// empty content is always valid
	}
	return nil
}

func validateRestrictionAttributes(schema *parser.Schema, baseCT *types.ComplexType, restrictionAttrs []*types.AttributeDecl, context string) error {
	if baseCT == nil {
		return nil
	}
	baseAttrMap := collectEffectiveAttributeUses(schema, baseCT)
	baseAnyAttr := collectAnyAttributeFromType(schema, baseCT)
	for _, restrictionAttr := range restrictionAttrs {
		effectiveRestriction := effectiveAttributeUse(schema, restrictionAttr)
		key := effectiveAttributeQNameForValidation(schema, effectiveRestriction)
		baseAttr, exists := baseAttrMap[key]
		if !exists {
			if baseAnyAttr == nil || !baseAnyAttr.AllowsQName(key) {
				return fmt.Errorf("%s: attribute '%s' not present in base type", context, restrictionAttr.Name.Local)
			}
			continue
		}
		effectiveBase := effectiveAttributeUse(schema, baseAttr)
		if effectiveBase.HasFixed {
			if !effectiveRestriction.HasFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
			baseFixed := effectiveBase.Fixed
			restrFixed := effectiveRestriction.Fixed
			if effectiveBase.Type != nil {
				baseFixed = types.NormalizeWhiteSpace(effectiveBase.Fixed, effectiveBase.Type)
				restrFixed = types.NormalizeWhiteSpace(effectiveRestriction.Fixed, effectiveBase.Type)
			}
			if baseFixed != restrFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
		}
		if effectiveBase.Use == types.Required && effectiveRestriction.Use != types.Required {
			return fmt.Errorf("%s: required attribute '%s' cannot be relaxed", context, restrictionAttr.Name.Local)
		}
		// attribute exists in base - type must match
		baseTypeQName := getTypeQName(effectiveBase.Type)
		restrictionTypeQName := getTypeQName(effectiveRestriction.Type)
		// skip comparison if either type is anonymous (empty QName)
		// anonymous types would require structural comparison which is complex
		if baseTypeQName.IsZero() || restrictionTypeQName.IsZero() {
			continue
		}
		if baseTypeQName != restrictionTypeQName {
			if !types.IsValidlyDerivedFrom(effectiveRestriction.Type, effectiveBase.Type) {
				return fmt.Errorf("%s: attribute '%s' type cannot be changed from '%s' to '%s' in restriction (only use can differ)", context, restrictionAttr.Name.Local, baseTypeQName, restrictionTypeQName)
			}
		}
	}
	return nil
}

func effectiveAttributeUse(schema *parser.Schema, attr *types.AttributeDecl) *types.AttributeDecl {
	if attr == nil || !attr.IsReference {
		return attr
	}
	target, ok := schema.AttributeDecls[attr.Name]
	if !ok {
		return attr
	}
	merged := *attr
	if merged.Type == nil {
		merged.Type = target.Type
	}
	if !merged.HasFixed && target.HasFixed {
		merged.Fixed = target.Fixed
		merged.HasFixed = true
	}
	if merged.Default == "" {
		merged.Default = target.Default
	}
	return &merged
}

// validateComplexContentStructure validates structural constraints of complex content
func validateComplexContentStructure(schema *parser.Schema, cc *types.ComplexContent) error {
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
				restrictionAttrs := slices.Clone(cc.Restriction.Attributes)
				restrictionAttrs = append(restrictionAttrs, collectAttributesFromGroups(schema, cc.Restriction.AttrGroups, nil)...)
				if err := validateRestrictionAttributes(schema, baseCT, restrictionAttrs, "complexContent restriction"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// validateSimpleContentStructure validates structural constraints of simple content
func validateSimpleContentStructure(schema *parser.Schema, sc *types.SimpleContent, isInline bool) error {
	// simple content doesn't have model groups
	if sc.Restriction != nil {
		// check if base type is valid for simpleContent restriction
		baseType, ok := schema.TypeDefs[sc.Restriction.Base]
		if ok {
			if _, isSimpleType := baseType.(*types.SimpleType); isSimpleType {
				return fmt.Errorf("simpleContent restriction cannot have simpleType base '%s'", sc.Restriction.Base)
			}
		} else if sc.Restriction.Base.Namespace == types.XSDNamespace {
			if types.GetBuiltin(types.TypeName(sc.Restriction.Base.Local)) != nil {
				return fmt.Errorf("simpleContent restriction cannot have simpleType base '%s'", sc.Restriction.Base)
			}
		}
		// per XSD spec: when a complexType is defined locally to an element (inline),
		// a simpleContent restriction with a simpleType base must have at least one facet.
		// empty restrictions (no facets) are not allowed in this context.
		if isInline {
			// check if base is a simpleType (not a complexType)
			if !ok || baseType == nil {
				// base type not found in schema - check if it's a built-in simpleType
				if sc.Restriction.Base.Namespace == types.XSDNamespace {
					// built-in type - check if it's a simpleType by checking if it's not a complex type name
					// for inline complexTypes, restrictions with simpleType bases must have facets
					if len(sc.Restriction.Facets) == 0 {
						return fmt.Errorf("simpleContent restriction in inline complexType cannot restrict simpleType '%s' without facets", sc.Restriction.Base)
					}
				}
			} else {
				// base type is resolved - check if it's a simpleType
				if _, isSimpleType := baseType.(*types.SimpleType); isSimpleType {
					// restriction with simpleType base must have at least one facet
					if len(sc.Restriction.Facets) == 0 {
						return fmt.Errorf("simpleContent restriction in inline complexType cannot restrict simpleType '%s' without facets", sc.Restriction.Base)
					}
				}
			}
		}
		if ok {
			if baseCT, ok := baseType.(*types.ComplexType); ok {
				// base must have simpleContent or be anyType
				if baseCT.QName.Local != "anyType" {
					if _, isSimpleContent := baseCT.Content().(*types.SimpleContent); !isSimpleContent {
						return fmt.Errorf("simpleContent restriction cannot derive from complexType '%s' which does not have simpleContent", sc.Restriction.Base)
					}
				}
			}
			// if it's a SimpleType, that's always valid for simpleContent restriction (unless inline with no facets, checked above)
		}
		if sc.Restriction.SimpleType != nil {
			baseSimpleType, baseQName := resolveSimpleContentBaseType(schema, sc.Restriction.Base)
			if baseSimpleType != nil {
				if sc.Restriction.SimpleType.List != nil || sc.Restriction.SimpleType.Union != nil {
					if baseQName.Namespace != types.XSDNamespace || baseQName.Local != string(types.TypeNameAnySimpleType) {
						return fmt.Errorf("simpleContent restriction simpleType is not derived from base type '%s'", baseQName)
					}
				} else if sc.Restriction.SimpleType.Restriction != nil {
					nestedBase := resolveSimpleTypeRestrictionBase(schema, sc.Restriction.SimpleType, sc.Restriction.SimpleType.Restriction)
					if nestedBase != nil && !types.IsValidlyDerivedFrom(nestedBase, baseSimpleType) {
						return fmt.Errorf("simpleContent restriction simpleType is not derived from base type '%s'", baseQName)
					}
				}
			}
		}
		if err := validateSimpleContentRestrictionFacets(schema, sc.Restriction); err != nil {
			return err
		}
		if ok {
			if baseCT, ok := baseType.(*types.ComplexType); ok {
				restrictionAttrs := slices.Clone(sc.Restriction.Attributes)
				restrictionAttrs = append(restrictionAttrs, collectAttributesFromGroups(schema, sc.Restriction.AttrGroups, nil)...)
				if err := validateRestrictionAttributes(schema, baseCT, restrictionAttrs, "simpleContent restriction"); err != nil {
					return err
				}
			}
		}
	}
	if sc.Extension != nil {
		// check if base type is valid for simpleContent extension
		baseType, ok := schema.TypeDefs[sc.Extension.Base]
		if ok {
			if baseCT, ok := baseType.(*types.ComplexType); ok {
				// base must have simpleContent
				if _, isSimpleContent := baseCT.Content().(*types.SimpleContent); !isSimpleContent {
					return fmt.Errorf("simpleContent extension cannot derive from complexType '%s' which does not have simpleContent", sc.Extension.Base)
				}
			}
			// if it's a SimpleType, that's always valid for simpleContent extension
		}
		if sc.Extension.Base.Namespace == types.XSDNamespace && sc.Extension.Base.Local == string(types.TypeNameAnyType) {
			return fmt.Errorf("simpleContent extension cannot have base type anyType")
		}
	}
	return nil
}
