package resolver

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

type idValuePolicy int

const (
	idValuesAllowed idValuePolicy = iota
	idValuesDisallowed
)

// validateDefaultOrFixedValueWithResolvedType validates a default/fixed value after type resolution.
func validateDefaultOrFixedValueWithResolvedType(schema *parser.Schema, value string, typ types.Type) error {
	return validateDefaultOrFixedValueWithResolvedTypeVisited(schema, value, typ, make(map[types.Type]bool))
}

func validateDefaultOrFixedValueWithResolvedTypeVisited(schema *parser.Schema, value string, typ types.Type, visited map[types.Type]bool) error {
	return validateDefaultOrFixedValueResolved(schema, value, typ, visited, idValuesDisallowed)
}

func validateDefaultOrFixedValueResolved(schema *parser.Schema, value string, typ types.Type, visited map[types.Type]bool, policy idValuePolicy) error {
	if typ == nil {
		return nil
	}
	if visited[typ] {
		return nil
	}
	visited[typ] = true
	defer delete(visited, typ)

	if ct, ok := typ.(*types.ComplexType); ok {
		textType := getComplexTypeTextType(schema, ct)
		if textType != nil {
			return validateDefaultOrFixedValueResolved(schema, value, textType, visited, policy)
		}
		return nil
	}

	normalizedValue := types.NormalizeWhiteSpace(value, typ)

	if typ.IsBuiltin() {
		bt := types.GetBuiltinNS(typ.Name().Namespace, typ.Name().Local)
		if bt != nil {
			if policy == idValuesDisallowed && schemacheck.IsIDOnlyType(typ.Name()) {
				return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
			}
			if err := bt.Validate(normalizedValue); err != nil {
				return err
			}
		}
		return nil
	}

	if st, ok := typ.(*types.SimpleType); ok {
		if policy == idValuesDisallowed && schemacheck.IsIDOnlyDerivedType(st) {
			return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
		}
		switch st.Variety() {
		case types.UnionVariety:
			memberTypes := resolveUnionMemberTypes(schema, st)
			if len(memberTypes) == 0 {
				return fmt.Errorf("union type '%s' has no member types", typ.Name().Local)
			}
			for _, member := range memberTypes {
				if err := validateDefaultOrFixedValueResolved(schema, normalizedValue, member, visited, idValuesAllowed); err == nil {
					return nil
				}
			}
			return fmt.Errorf("value '%s' does not match any member type of union '%s'", normalizedValue, typ.Name().Local)
		case types.ListVariety:
			itemType := resolveListItemType(schema, st)
			if itemType == nil {
				return nil
			}
			for item := range types.FieldsXMLWhitespaceSeq(normalizedValue) {
				if err := validateDefaultOrFixedValueResolved(schema, item, itemType, visited, policy); err != nil {
					return err
				}
			}
			return nil
		default:
			if err := st.Validate(normalizedValue); err != nil {
				return err
			}
			if err := validateValueAgainstFacets(normalizedValue, st); err != nil {
				return err
			}
			return nil
		}
	}

	return nil
}

func resolveUnionMemberTypes(schema *parser.Schema, st *types.SimpleType) []types.Type {
	if st == nil {
		return nil
	}
	if len(st.MemberTypes) > 0 {
		return st.MemberTypes
	}
	if st.Union == nil {
		return nil
	}
	memberTypes := make([]types.Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
	for _, inline := range st.Union.InlineTypes {
		memberTypes = append(memberTypes, inline)
	}
	for _, memberQName := range st.Union.MemberTypes {
		if member := schemacheck.ResolveSimpleTypeReference(schema, memberQName); member != nil {
			memberTypes = append(memberTypes, member)
		}
	}
	return memberTypes
}

func resolveListItemType(schema *parser.Schema, st *types.SimpleType) types.Type {
	if st == nil || st.List == nil {
		return nil
	}
	if st.ItemType != nil {
		return st.ItemType
	}
	if st.List.InlineItemType != nil {
		return st.List.InlineItemType
	}
	if !st.List.ItemType.IsZero() {
		return schemacheck.ResolveSimpleTypeReference(schema, st.List.ItemType)
	}
	return nil
}

// validateValueAgainstFacets validates a value against all facets of a simple type.
func validateValueAgainstFacets(value string, st *types.SimpleType) error {
	if st == nil || st.Restriction == nil {
		return nil
	}

	for _, facetIface := range st.Restriction.Facets {
		facet, ok := facetIface.(types.Facet)
		if !ok {
			continue
		}

		typedValue := types.TypedValueForFacet(value, st)

		if err := facet.Validate(typedValue, st); err != nil {
			return fmt.Errorf("facet '%s' violation: %w", facet.Name(), err)
		}
	}

	return nil
}

// getComplexTypeTextType returns the text content type for a complex type with simple content.
func getComplexTypeTextType(schema *parser.Schema, ct *types.ComplexType) types.Type {
	content := ct.Content()
	sc, ok := content.(*types.SimpleContent)
	if !ok {
		return nil
	}

	var baseQName types.QName
	if sc.Extension != nil {
		baseQName = sc.Extension.Base
	} else if sc.Restriction != nil {
		baseQName = sc.Restriction.Base
	}

	if baseQName.IsZero() {
		return nil
	}

	// check if base is a built-in type.
	if bt := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local); bt != nil {
		return bt
	}

	// try to resolve from schema.
	if resolvedType, ok := schema.TypeDefs[baseQName]; ok {
		return resolvedType
	}

	return nil
}
