package semanticcheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typechain"
)

// validateSimpleContentStructure validates structural constraints of simple content.
func validateSimpleContentStructure(schema *parser.Schema, sc *model.SimpleContent, context typeDefinitionContext) error {
	if sc.Restriction != nil {
		baseType, baseOK := typechain.LookupType(schema, sc.Restriction.Base)
		if baseOK {
			if _, isSimpleType := baseType.(*model.SimpleType); isSimpleType {
				return fmt.Errorf("simpleContent restriction cannot have simpleType base '%s'", sc.Restriction.Base)
			}
		} else if sc.Restriction.Base.Namespace == model.XSDNamespace {
			if builtins.Get(builtins.TypeName(sc.Restriction.Base.Local)) != nil {
				return fmt.Errorf("simpleContent restriction cannot have simpleType base '%s'", sc.Restriction.Base)
			}
		}
		if context == typeDefinitionInline {
			if !baseOK || baseType == nil {
				if sc.Restriction.Base.Namespace == model.XSDNamespace {
					if len(sc.Restriction.Facets) == 0 {
						return fmt.Errorf("simpleContent restriction in inline complexType cannot restrict simpleType '%s' without facets", sc.Restriction.Base)
					}
				}
			} else {
				if _, isSimpleType := baseType.(*model.SimpleType); isSimpleType {
					if len(sc.Restriction.Facets) == 0 {
						return fmt.Errorf("simpleContent restriction in inline complexType cannot restrict simpleType '%s' without facets", sc.Restriction.Base)
					}
				}
			}
		}
		if baseCT, ok := baseType.(*model.ComplexType); ok {
			if baseCT.QName.Local != "anyType" {
				if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); !isSimpleContent {
					return fmt.Errorf("simpleContent restriction cannot derive from complexType '%s' which does not have simpleContent", sc.Restriction.Base)
				}
			}
		}
		if sc.Restriction.SimpleType != nil {
			baseSimpleType, baseQName := resolveSimpleContentBaseType(schema, sc.Restriction.Base)
			if baseSimpleType != nil {
				if sc.Restriction.SimpleType.List != nil || sc.Restriction.SimpleType.Union != nil {
					if baseQName.Namespace != model.XSDNamespace || baseQName.Local != string(model.TypeNameAnySimpleType) {
						return fmt.Errorf("simpleContent restriction simpleType is not derived from base type '%s'", baseQName)
					}
				} else if sc.Restriction.SimpleType.Restriction != nil {
					nestedBase := resolveSimpleTypeRestrictionBase(schema, sc.Restriction.SimpleType, sc.Restriction.SimpleType.Restriction)
					if nestedBase != nil && !model.IsValidlyDerivedFrom(nestedBase, baseSimpleType) {
						return fmt.Errorf("simpleContent restriction simpleType is not derived from base type '%s'", baseQName)
					}
				}
			}
		}
		if err := validateSimpleContentRestrictionFacets(schema, sc.Restriction); err != nil {
			return err
		}
		if baseCT, ok := baseType.(*model.ComplexType); ok {
			restrictionAttrs := slices.Clone(sc.Restriction.Attributes)
			restrictionAttrs = append(restrictionAttrs, collectAttributesFromGroups(schema, sc.Restriction.AttrGroups)...)
			if err := validateRestrictionAttributes(schema, baseCT, restrictionAttrs, "simpleContent restriction"); err != nil {
				return err
			}
		}
	}
	if sc.Extension != nil {
		baseType, _ := typechain.LookupType(schema, sc.Extension.Base)
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
