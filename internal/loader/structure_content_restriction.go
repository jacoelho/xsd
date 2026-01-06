package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

func validateRestrictionAttributes(schema *schema.Schema, baseCT *types.ComplexType, restrictionAttrs []*types.AttributeDecl, context string) error {
	if baseCT == nil {
		return nil
	}
	baseAttrs := collectAllAttributesForValidation(schema, baseCT)
	baseAttrMap := make(map[types.QName]*types.AttributeDecl)
	for _, baseAttr := range baseAttrs {
		baseAttrMap[effectiveAttributeQNameForValidation(schema, baseAttr)] = baseAttr
	}
	baseAnyAttr := collectAnyAttributeFromType(schema, baseCT)
	for _, restrictionAttr := range restrictionAttrs {
		key := effectiveAttributeQNameForValidation(schema, restrictionAttr)
		baseAttr, exists := baseAttrMap[key]
		if !exists {
			if baseAnyAttr == nil || !baseAnyAttr.AllowsQName(key) {
				return fmt.Errorf("%s: attribute '%s' not present in base type", context, restrictionAttr.Name.Local)
			}
			continue
		}
		if baseAttr.HasFixed {
			if !restrictionAttr.HasFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
			baseFixed := baseAttr.Fixed
			restrFixed := restrictionAttr.Fixed
			if baseAttr.Type != nil {
				baseFixed = types.NormalizeWhiteSpace(baseAttr.Fixed, baseAttr.Type)
				restrFixed = types.NormalizeWhiteSpace(restrictionAttr.Fixed, baseAttr.Type)
			}
			if baseFixed != restrFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
		}
		if baseAttr.Use == types.Required && restrictionAttr.Use != types.Required {
			return fmt.Errorf("%s: required attribute '%s' cannot be relaxed", context, restrictionAttr.Name.Local)
		}
		// Attribute exists in base - type must match
		baseTypeQName := getTypeQName(baseAttr.Type)
		restrictionTypeQName := getTypeQName(restrictionAttr.Type)
		// Skip comparison if either type is anonymous (empty QName)
		// Anonymous types would require structural comparison which is complex
		if baseTypeQName.IsZero() || restrictionTypeQName.IsZero() {
			continue
		}
		if baseTypeQName != restrictionTypeQName {
			if !types.IsValidlyDerivedFrom(restrictionAttr.Type, baseAttr.Type) {
				return fmt.Errorf("%s: attribute '%s' type cannot be changed from '%s' to '%s' in restriction (only use can differ)", context, restrictionAttr.Name.Local, baseTypeQName, restrictionTypeQName)
			}
		}
	}
	return nil
}
