package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/facetvalue"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	facetengine "github.com/jacoelho/xsd/internal/schemafacet"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

func validateValueAgainstTypeWithFacets(schema *parser.Schema, value string, typ model.Type, context map[string]string, visited map[model.Type]bool) error {
	if typ == nil {
		return nil
	}
	if visited[typ] {
		return fmt.Errorf("circular type reference")
	}
	visited[typ] = true
	defer delete(visited, typ)

	if ct, ok := typ.(*model.ComplexType); ok {
		sc, ok := ct.Content().(*model.SimpleContent)
		if !ok {
			return nil
		}
		baseType := typeresolve.ResolveSimpleContentBaseTypeFromContent(schema, sc)
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

	normalized := model.NormalizeWhiteSpace(value, typ)

	if facetvalue.IsQNameOrNotationType(typ) {
		if context == nil {
			return fmt.Errorf("namespace context unavailable for QName/NOTATION value")
		}
		if err := facetengine.ValidateQNameContext(normalized, context); err != nil {
			return err
		}
	}

	if typ.IsBuiltin() {
		bt := builtins.GetNS(typ.Name().Namespace, typ.Name().Local)
		if bt == nil {
			return nil
		}
		if err := bt.Validate(normalized); err != nil {
			return err
		}
		return nil
	}

	st, ok := typ.(*model.SimpleType)
	if !ok {
		return nil
	}

	switch st.Variety() {
	case model.UnionVariety:
		memberTypes := typeresolve.ResolveUnionMemberTypes(schema, st)
		if len(memberTypes) == 0 {
			return fmt.Errorf("union has no member types")
		}
		for _, member := range memberTypes {
			if err := validateValueAgainstTypeWithFacets(schema, normalized, member, context, visited); err == nil {
				return facetengine.ValidateSimpleTypeFacets(schema, st, normalized, context, convertDeferredFacet)
			}
		}
		return fmt.Errorf("value %q does not match any member type of union", normalized)
	case model.ListVariety:
		itemType := typeresolve.ResolveListItemType(schema, st)
		if itemType == nil {
			return nil
		}
		for item := range model.FieldsXMLWhitespaceSeq(normalized) {
			if err := validateValueAgainstTypeWithFacets(schema, item, itemType, context, visited); err != nil {
				return err
			}
		}
		return facetengine.ValidateSimpleTypeFacets(schema, st, normalized, context, convertDeferredFacet)
	default:
		if !facetvalue.IsQNameOrNotationType(st) {
			if err := st.Validate(normalized); err != nil {
				return err
			}
		}
		return facetengine.ValidateSimpleTypeFacets(schema, st, normalized, context, convertDeferredFacet)
	}
}
