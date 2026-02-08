package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func validateRestrictionAttributes(schema *parser.Schema, baseCT *types.ComplexType, restrictionAttrs []*types.AttributeDecl, context string) error {
	if baseCT == nil {
		return nil
	}
	baseAttrMap := collectEffectiveAttributeUses(schema, baseCT)
	baseAnyAttr, err := collectAnyAttributeFromType(schema, baseCT)
	if err != nil {
		return err
	}
	for _, restrictionAttr := range restrictionAttrs {
		effectiveRestriction := effectiveAttributeUse(schema, restrictionAttr)
		key := typeops.EffectiveAttributeQName(schema, effectiveRestriction)
		baseAttr, exists := baseAttrMap[key]
		if !exists {
			if baseAnyAttr == nil || !baseAnyAttr.AllowsQName(key) {
				return fmt.Errorf("%s: attribute '%s' not present in base type", context, restrictionAttr.Name.Local)
			}
			continue
		}
		effectiveBase := effectiveAttributeUse(schema, baseAttr)
		if effectiveBase.Use == types.Required && effectiveRestriction.Use != types.Required {
			return fmt.Errorf("%s: required attribute '%s' cannot be relaxed", context, restrictionAttr.Name.Local)
		}
		if effectiveRestriction.Use == types.Prohibited {
			continue
		}
		if effectiveBase.HasFixed {
			if !effectiveRestriction.HasFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
			baseType := ResolveTypeReference(schema, effectiveBase.Type, typeops.TypeReferenceAllowMissing)
			if baseType == nil {
				baseType = effectiveBase.Type
			}
			baseFixed := normalizeFixedValue(effectiveBase.Fixed, baseType)
			restrFixed := normalizeFixedValue(effectiveRestriction.Fixed, baseType)
			if baseFixed != restrFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
		}
		baseTypeQName := getTypeQName(effectiveBase.Type)
		restrictionTypeQName := getTypeQName(effectiveRestriction.Type)
		if baseTypeQName.IsZero() || restrictionTypeQName.IsZero() {
			continue
		}
		if baseTypeQName != restrictionTypeQName {
			baseType := ResolveTypeReference(schema, effectiveBase.Type, typeops.TypeReferenceAllowMissing)
			restrictionType := ResolveTypeReference(schema, effectiveRestriction.Type, typeops.TypeReferenceAllowMissing)
			if baseType == nil {
				baseType = effectiveBase.Type
			}
			if restrictionType == nil {
				restrictionType = effectiveRestriction.Type
			}
			if !isRestrictionDerivedFrom(schema, restrictionType, baseType) {
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
	if !merged.HasDefault && target.HasDefault {
		merged.Default = target.Default
		merged.HasDefault = true
	}
	return &merged
}
