package semanticcheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

// validateSimpleContentStructure validates structural constraints of simple content.
func validateSimpleContentStructure(schema *parser.Schema, sc *model.SimpleContent, context typeDefinitionContext) error {
	if sc.Restriction != nil {
		baseType, baseOK := semantics.LookupType(schema, sc.Restriction.Base)
		if err := validateSimpleContentRestrictionBase(sc.Restriction.Base, baseType, baseOK); err != nil {
			return err
		}
		if err := validateInlineSimpleContentRestriction(context, sc.Restriction, baseType, baseOK); err != nil {
			return err
		}
		if err := validateSimpleContentRestrictionComplexBase(sc.Restriction.Base, baseType); err != nil {
			return err
		}
		if err := validateSimpleContentRestrictionSimpleType(schema, sc.Restriction); err != nil {
			return err
		}
		if err := validateSimpleContentRestrictionFacets(schema, sc.Restriction); err != nil {
			return err
		}
		if baseCT, ok := baseType.(*model.ComplexType); ok {
			if err := validateSimpleContentRestrictionAttributes(schema, baseCT, sc.Restriction); err != nil {
				return err
			}
		}
	}
	if sc.Extension != nil {
		baseType, _ := semantics.LookupType(schema, sc.Extension.Base)
		if baseCT, ok := baseType.(*model.ComplexType); ok {
			if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); !isSimpleContent {
				return fmt.Errorf("simpleContent extension cannot derive from complexType '%s' which does not have simpleContent", sc.Extension.Base)
			}
		}
		if sc.Extension.Base.Namespace == model.XSDNamespace && sc.Extension.Base.Local == string(model.TypeNameAnyType) {
			return fmt.Errorf("simpleContent extension cannot have base type anyType")
		}
	}
	return nil
}

func validateSimpleContentRestrictionBase(baseQName model.QName, baseType model.Type, baseOK bool) error {
	if baseOK {
		if _, isSimpleType := baseType.(*model.SimpleType); isSimpleType {
			return fmt.Errorf("simpleContent restriction cannot have simpleType base '%s'", baseQName)
		}
		return nil
	}
	if baseQName.Namespace == model.XSDNamespace && model.GetBuiltin(model.TypeName(baseQName.Local)) != nil {
		return fmt.Errorf("simpleContent restriction cannot have simpleType base '%s'", baseQName)
	}
	return nil
}

func validateInlineSimpleContentRestriction(context typeDefinitionContext, restriction *model.Restriction, baseType model.Type, baseOK bool) error {
	if context != typeDefinitionInline || len(restriction.Facets) > 0 {
		return nil
	}
	if !baseOK || baseType == nil {
		if restriction.Base.Namespace == model.XSDNamespace {
			return fmt.Errorf("simpleContent restriction in inline complexType cannot restrict simpleType '%s' without facets", restriction.Base)
		}
		return nil
	}
	if _, isSimpleType := baseType.(*model.SimpleType); isSimpleType {
		return fmt.Errorf("simpleContent restriction in inline complexType cannot restrict simpleType '%s' without facets", restriction.Base)
	}
	return nil
}

func validateSimpleContentRestrictionComplexBase(baseQName model.QName, baseType model.Type) error {
	baseCT, ok := baseType.(*model.ComplexType)
	if !ok || baseCT.QName.Local == "anyType" {
		return nil
	}
	if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); isSimpleContent {
		return nil
	}
	return fmt.Errorf("simpleContent restriction cannot derive from complexType '%s' which does not have simpleContent", baseQName)
}

func validateSimpleContentRestrictionSimpleType(schema *parser.Schema, restriction *model.Restriction) error {
	if restriction.SimpleType == nil {
		return nil
	}
	baseSimpleType, baseQName := resolveSimpleContentBaseType(schema, restriction.Base)
	if baseSimpleType == nil {
		return nil
	}
	if restriction.SimpleType.List != nil || restriction.SimpleType.Union != nil {
		if baseQName.Namespace != model.XSDNamespace || baseQName.Local != string(model.TypeNameAnySimpleType) {
			return fmt.Errorf("simpleContent restriction simpleType is not derived from base type '%s'", baseQName)
		}
		return nil
	}
	if restriction.SimpleType.Restriction == nil {
		return nil
	}
	nestedBase := resolveSimpleTypeRestrictionBase(schema, restriction.SimpleType, restriction.SimpleType.Restriction)
	if nestedBase != nil && !model.IsValidlyDerivedFrom(nestedBase, baseSimpleType) {
		return fmt.Errorf("simpleContent restriction simpleType is not derived from base type '%s'", baseQName)
	}
	return nil
}

func validateSimpleContentRestrictionAttributes(schema *parser.Schema, baseCT *model.ComplexType, restriction *model.Restriction) error {
	restrictionAttrs := slices.Clone(restriction.Attributes)
	restrictionAttrs = append(restrictionAttrs, collectAttributesFromGroups(schema, restriction.AttrGroups)...)
	return validateRestrictionAttributes(schema, baseCT, restrictionAttrs, "simpleContent restriction")
}
