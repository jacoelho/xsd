package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateSimpleContentStructure validates structural constraints of simple content
func validateSimpleContentStructure(schema *schema.Schema, sc *types.SimpleContent, isInline bool) error {
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
				restrictionAttrs := append([]*types.AttributeDecl(nil), sc.Restriction.Attributes...)
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