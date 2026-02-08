package semanticcheck

import (
	"fmt"

	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func validateValueAgainstTypeWithFacets(schema *parser.Schema, value string, typ types.Type, context map[string]string, visited map[types.Type]bool) error {
	if typ == nil {
		return nil
	}
	if visited[typ] {
		return fmt.Errorf("circular type reference")
	}
	visited[typ] = true
	defer delete(visited, typ)

	if ct, ok := typ.(*types.ComplexType); ok {
		sc, ok := ct.Content().(*types.SimpleContent)
		if !ok {
			return nil
		}
		baseType := typeops.ResolveSimpleContentBaseTypeFromContent(schema, sc)
		if baseType == nil {
			return nil
		}
		if sc.Restriction != nil {
			if err := facetengine.ValidateRestrictionFacets(schema, sc.Restriction, baseType, value, context, convertDeferredFacet); err != nil {
				return err
			}
		}
		return validateValueAgainstTypeWithFacets(schema, value, baseType, context, visited)
	}

	normalized := types.NormalizeWhiteSpace(value, typ)

	if types.IsQNameOrNotationType(typ) {
		if context == nil {
			return fmt.Errorf("namespace context unavailable for QName/NOTATION value")
		}
		if err := facetengine.ValidateQNameContext(normalized, context); err != nil {
			return err
		}
	}

	if typ.IsBuiltin() {
		bt := types.GetBuiltinNS(typ.Name().Namespace, typ.Name().Local)
		if bt == nil {
			return nil
		}
		if err := bt.Validate(normalized); err != nil {
			return err
		}
		return nil
	}

	st, ok := typ.(*types.SimpleType)
	if !ok {
		return nil
	}

	switch st.Variety() {
	case types.UnionVariety:
		memberTypes := typeops.ResolveUnionMemberTypes(schema, st)
		if len(memberTypes) == 0 {
			return fmt.Errorf("union has no member types")
		}
		for _, member := range memberTypes {
			if err := validateValueAgainstTypeWithFacets(schema, normalized, member, context, visited); err == nil {
				return facetengine.ValidateSimpleTypeFacets(schema, st, normalized, context, convertDeferredFacet)
			}
		}
		return fmt.Errorf("value %q does not match any member type of union", normalized)
	case types.ListVariety:
		itemType := typeops.ResolveListItemType(schema, st)
		if itemType == nil {
			return nil
		}
		for item := range types.FieldsXMLWhitespaceSeq(normalized) {
			if err := validateValueAgainstTypeWithFacets(schema, item, itemType, context, visited); err != nil {
				return err
			}
		}
		return facetengine.ValidateSimpleTypeFacets(schema, st, normalized, context, convertDeferredFacet)
	default:
		if !types.IsQNameOrNotationType(st) {
			if err := st.Validate(normalized); err != nil {
				return err
			}
		}
		return facetengine.ValidateSimpleTypeFacets(schema, st, normalized, context, convertDeferredFacet)
	}
}
