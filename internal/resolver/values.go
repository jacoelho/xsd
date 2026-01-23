package resolver

import (
	"fmt"
	"slices"

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
func validateDefaultOrFixedValueWithResolvedType(schema *parser.Schema, value string, typ types.Type, context map[string]string) error {
	return validateDefaultOrFixedValueWithResolvedTypeVisited(schema, value, typ, context, make(map[types.Type]bool))
}

func validateDefaultOrFixedValueWithResolvedTypeVisited(schema *parser.Schema, value string, typ types.Type, context map[string]string, visited map[types.Type]bool) error {
	return validateDefaultOrFixedValueResolved(schema, value, typ, context, visited, idValuesDisallowed)
}

func validateDefaultOrFixedValueResolved(schema *parser.Schema, value string, typ types.Type, context map[string]string, visited map[types.Type]bool, policy idValuePolicy) error {
	if typ == nil {
		return nil
	}
	if visited[typ] {
		return nil
	}
	visited[typ] = true
	defer delete(visited, typ)

	if ct, ok := typ.(*types.ComplexType); ok {
		sc, ok := ct.Content().(*types.SimpleContent)
		if !ok {
			return nil
		}
		baseType := resolveSimpleContentBaseType(schema, sc)
		if baseType == nil {
			return nil
		}
		if sc.Restriction != nil {
			if err := validateDefaultOrFixedValueResolved(schema, value, baseType, context, visited, policy); err != nil {
				return err
			}
			normalized := types.NormalizeWhiteSpace(value, baseType)
			facets := collectRestrictionFacets(schema, sc.Restriction, baseType)
			return validateValueAgainstFacets(normalized, baseType, facets, context)
		}
		return validateDefaultOrFixedValueResolved(schema, value, baseType, context, visited, policy)
	}

	normalizedValue := types.NormalizeWhiteSpace(value, typ)

	if types.IsQNameOrNotationType(typ) {
		if _, err := types.ParseQNameValue(normalizedValue, nsContext); err != nil {
			return err
		}
	}

	if typ.IsBuiltin() {
		bt := types.GetBuiltinNS(typ.Name().Namespace, typ.Name().Local)
		if bt != nil {
			if policy == idValuesDisallowed && schemacheck.IsIDOnlyType(typ.Name()) {
				return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
			}
			if err := bt.Validate(normalizedValue); err != nil {
				return err
			}
			if isQNameOrNotationTypeValue(typ) {
				if err := validateQNameContext(normalizedValue, context); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if st, ok := typ.(*types.SimpleType); ok {
		if policy == idValuesDisallowed && schemacheck.IsIDOnlyDerivedType(schema, st) {
			return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
		}
		if memberTypes := resolveUnionMemberTypes(schema, st); len(memberTypes) > 0 {
			matched := false
			for _, member := range memberTypes {
				if err := validateDefaultOrFixedValueResolved(schema, normalizedValue, member, context, visited, idValuesAllowed); err == nil {
					facets := collectSimpleTypeFacets(schema, st, make(map[*types.SimpleType]bool))
					return validateValueAgainstFacets(normalizedValue, st, facets, context)
				}
			}
			return fmt.Errorf("value '%s' does not match any member type of union '%s'", normalizedValue, typ.Name().Local)
		case types.ListVariety:
			itemType := resolveListItemType(schema, st)
			if itemType != nil {
				for item := range types.FieldsXMLWhitespaceSeq(normalizedValue) {
					if err := validateDefaultOrFixedValueResolved(schema, item, itemType, context, visited, policy); err != nil {
						return err
					}
				}
			}
			facets := collectSimpleTypeFacets(schema, st, make(map[*types.SimpleType]bool))
			return validateValueAgainstFacets(normalizedValue, st, facets, context)
		default:
			if err := st.Validate(normalizedValue); err != nil {
				return err
			}
			if isQNameOrNotationTypeValue(st) {
				if err := validateQNameContext(normalizedValue, context); err != nil {
					return err
				}
			}
			facets := collectSimpleTypeFacets(schema, st, make(map[*types.SimpleType]bool))
			return validateValueAgainstFacets(normalizedValue, st, facets, context)
		}

		if err := st.Validate(normalizedValue); err != nil {
			return err
		}
		if err := validateValueAgainstFacets(schema, normalizedValue, st, nsContext); err != nil {
			return err
		}
		return nil
	}

	return nil
}

func resolveUnionMemberTypes(schema *parser.Schema, st *types.SimpleType) []types.Type {
	return resolveUnionMemberTypesVisited(schema, st, make(map[*types.SimpleType]bool))
}

func resolveUnionMemberTypesVisited(schema *parser.Schema, st *types.SimpleType, visited map[*types.SimpleType]bool) []types.Type {
	if st == nil {
		return nil
	}
	if visited[st] {
		return nil
	}
	visited[st] = true
	defer delete(visited, st)

	if len(st.MemberTypes) > 0 {
		return st.MemberTypes
	}
	if st.Union != nil {
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
	if base := resolveBaseSimpleType(schema, st); base != nil {
		return resolveUnionMemberTypesVisited(schema, base, visited)
	}
	return nil
}

func resolveListItemType(schema *parser.Schema, st *types.SimpleType) types.Type {
	if st == nil || st.List == nil {
		if st == nil {
			return nil
		}
		if itemType, ok := types.ListItemType(st); ok {
			return itemType
		}
		if st.Restriction != nil && !st.Restriction.Base.IsZero() {
			if base := schemacheck.ResolveSimpleTypeReference(schema, st.Restriction.Base); base != nil {
				if itemType, ok := types.ListItemType(base); ok {
					return itemType
				}
			}
		}
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
	if itemType, ok := types.ListItemType(st); ok {
		return itemType
	}
	return nil
}

func validateValueAgainstFacets(value string, baseType types.Type, facets []types.Facet, context map[string]string) error {
	if len(facets) == 0 {
		return nil
	}

	var typedValue types.TypedValue
	for _, facet := range facets {
		if shouldSkipLengthFacet(baseType, facet) {
			continue
		}
		if enumFacet, ok := facet.(*types.Enumeration); ok && isQNameOrNotationTypeValue(baseType) && !isListType(baseType) {
			qname, err := types.ParseQNameValue(value, context)
			if err != nil {
				return err
			}
			allowed, err := enumFacet.ResolveQNameValues()
			if err != nil {
				return err
			}
			if slices.Contains(allowed, qname) {
				continue
			}
			return fmt.Errorf("value %s not in enumeration: %s", value, types.FormatEnumerationValues(enumFacet.Values))
		}
		if lexicalFacet, ok := facet.(types.LexicalValidator); ok {
			if err := lexicalFacet.ValidateLexical(value, baseType); err != nil {
				return fmt.Errorf("facet '%s' violation: %w", facet.Name(), err)
			}
			continue
		}
		if typedValue == nil {
			typedValue = types.TypedValueForFacet(value, baseType)
		}
		if err := facet.Validate(typedValue, baseType); err != nil {
			return fmt.Errorf("facet '%s' violation: %w", facet.Name(), err)
		}
	}

	return nil
}

func collectSimpleTypeFacets(schema *parser.Schema, st *types.SimpleType, visited map[*types.SimpleType]bool) []types.Facet {
	if st == nil {
		return nil
	}
	if visited[st] {
		return nil
	}
	visited[st] = true
	defer delete(visited, st)

	var result []types.Facet
	if st.ResolvedBase != nil {
		if baseST, ok := st.ResolvedBase.(*types.SimpleType); ok {
			result = append(result, collectSimpleTypeFacets(schema, baseST, visited)...)
		}
	} else if st.Restriction != nil && !st.Restriction.Base.IsZero() {
		if base := schemacheck.ResolveSimpleTypeReference(schema, st.Restriction.Base); base != nil {
			if baseST, ok := base.(*types.SimpleType); ok {
				result = append(result, collectSimpleTypeFacets(schema, baseST, visited)...)
			}
		}
	}

	if st.Restriction != nil {
		var baseType types.Type
		if st.ResolvedBase != nil {
			baseType = st.ResolvedBase
		} else if !st.Restriction.Base.IsZero() {
			baseType = schemacheck.ResolveSimpleTypeReference(schema, st.Restriction.Base)
		}
		result = append(result, collectRestrictionFacets(schema, st.Restriction, baseType)...)
	}

	return result
}

func collectRestrictionFacets(schema *parser.Schema, restriction *types.Restriction, baseType types.Type) []types.Facet {
	if restriction == nil || len(restriction.Facets) == 0 {
		return nil
	}

	var (
		result       []types.Facet
		stepPatterns []*types.Pattern
	)

	for _, facetIface := range restriction.Facets {
		switch facet := facetIface.(type) {
		case *types.Pattern:
			if err := facet.ValidateSyntax(); err != nil {
				continue
			}
			stepPatterns = append(stepPatterns, facet)
		case types.Facet:
			if compilable, ok := facet.(interface{ ValidateSyntax() error }); ok {
				if err := compilable.ValidateSyntax(); err != nil {
					continue
				}
			}
			result = append(result, facet)
		case *types.DeferredFacet:
			if baseType == nil {
				baseType = schemacheck.ResolveSimpleTypeReference(schema, restriction.Base)
			}
			if baseType == nil {
				continue
			}
			resolved, err := convertDeferredFacet(facet, baseType)
			if err != nil || resolved == nil {
				continue
			}
			result = append(result, resolved)
		}
	}

	if len(stepPatterns) == 1 {
		result = append(result, stepPatterns[0])
	} else if len(stepPatterns) > 1 {
		result = append(result, &types.PatternSet{Patterns: stepPatterns})
	}

	return result
}

func resolveSimpleContentBaseType(schema *parser.Schema, sc *types.SimpleContent) types.Type {
	if sc == nil {
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
	if bt := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local); bt != nil {
		return bt
	}
	if resolvedType, ok := schema.TypeDefs[baseQName]; ok {
		return resolvedType
	}
	return nil
}

func convertDeferredFacet(df *types.DeferredFacet, baseType types.Type) (types.Facet, error) {
	if df == nil || baseType == nil {
		return nil, nil
	}
	switch df.FacetName {
	case "minInclusive":
		return types.NewMinInclusive(df.FacetValue, baseType)
	case "maxInclusive":
		return types.NewMaxInclusive(df.FacetValue, baseType)
	case "minExclusive":
		return types.NewMinExclusive(df.FacetValue, baseType)
	case "maxExclusive":
		return types.NewMaxExclusive(df.FacetValue, baseType)
	default:
		return nil, fmt.Errorf("unknown deferred facet type: %s", df.FacetName)
	}
}

func validateQNameContext(value string, context map[string]string) error {
	_, err := types.ParseQNameValue(value, context)
	return err
}

func isQNameOrNotationTypeValue(typ types.Type) bool {
	if typ == nil {
		return false
	}
	switch t := typ.(type) {
	case *types.SimpleType:
		return t.IsQNameOrNotationType()
	case *types.BuiltinType:
		return t.IsQNameOrNotationType()
	default:
		if prim := typ.PrimitiveType(); prim != nil {
			switch p := prim.(type) {
			case *types.SimpleType:
				return p.IsQNameOrNotationType()
			case *types.BuiltinType:
				return p.IsQNameOrNotationType()
			}
		}
		return false
	}
}

func isListType(typ types.Type) bool {
	switch t := typ.(type) {
	case *types.SimpleType:
		return t.Variety() == types.ListVariety || t.List != nil
	case *types.BuiltinType:
		name := t.Name().Local
		return name == "NMTOKENS" || name == "IDREFS" || name == "ENTITIES"
	default:
		return false
	}
}

func shouldSkipLengthFacet(baseType types.Type, facet types.Facet) bool {
	if !isLengthFacet(facet) {
		return false
	}
	if isListType(baseType) {
		return false
	}
	return isQNameOrNotationTypeValue(baseType)
}

func isLengthFacet(facet types.Facet) bool {
	switch facet.(type) {
	case *types.Length, *types.MinLength, *types.MaxLength:
		return true
	default:
		return false
	}
}
