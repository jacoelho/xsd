package validation

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

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
