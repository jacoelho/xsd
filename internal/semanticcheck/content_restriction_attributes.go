package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
)

func validateRestrictionAttributes(schema *parser.Schema, baseCT *model.ComplexType, restrictionAttrs []*model.AttributeDecl, context string) error {
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
			if baseAnyAttr == nil || !model.AllowsNamespace(
				baseAnyAttr.Namespace,
				baseAnyAttr.NamespaceList,
				baseAnyAttr.TargetNamespace,
				key.Namespace,
			) {
				return fmt.Errorf("%s: attribute '%s' not present in base type", context, restrictionAttr.Name.Local)
			}
			continue
		}
		effectiveBase := effectiveAttributeUse(schema, baseAttr)
		if effectiveBase.Use == model.Required && effectiveRestriction.Use != model.Required {
			return fmt.Errorf("%s: required attribute '%s' cannot be relaxed", context, restrictionAttr.Name.Local)
		}
		if effectiveRestriction.Use == model.Prohibited {
			continue
		}
		if effectiveBase.HasFixed {
			if !effectiveRestriction.HasFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
			baseType := typeops.ResolveTypeReference(schema, effectiveBase.Type, typeops.TypeReferenceAllowMissing)
			if baseType == nil {
				baseType = effectiveBase.Type
			}
			baseFixed := normalizeFixedValue(effectiveBase.Fixed, baseType)
			restrFixed := normalizeFixedValue(effectiveRestriction.Fixed, baseType)
			if baseFixed != restrFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
		}
		baseTypeQName := model.QName{}
		if effectiveBase.Type != nil {
			baseTypeQName = effectiveBase.Type.Name()
		}
		restrictionTypeQName := model.QName{}
		if effectiveRestriction.Type != nil {
			restrictionTypeQName = effectiveRestriction.Type.Name()
		}
		if baseTypeQName.IsZero() || restrictionTypeQName.IsZero() {
			continue
		}
		if baseTypeQName != restrictionTypeQName {
			baseType := typeops.ResolveTypeReference(schema, effectiveBase.Type, typeops.TypeReferenceAllowMissing)
			restrictionType := typeops.ResolveTypeReference(schema, effectiveRestriction.Type, typeops.TypeReferenceAllowMissing)
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

func effectiveAttributeUse(schema *parser.Schema, attr *model.AttributeDecl) *model.AttributeDecl {
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
