package resolver

import (
	"errors"
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

var errCircularReference = errors.New("circular type reference")

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
		return errCircularReference
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
		if _, err := types.ParseQNameValue(normalizedValue, context); err != nil {
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
			if types.IsQNameOrNotationType(typ) {
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
		switch st.Variety() {
		case types.UnionVariety:
			memberTypes := resolveUnionMemberTypes(schema, st)
			if len(memberTypes) > 0 {
				var firstErr error
				sawCycle := false
				for _, member := range memberTypes {
					if err := validateDefaultOrFixedValueResolved(schema, normalizedValue, member, context, visited, idValuesAllowed); err == nil {
						facets := collectSimpleTypeFacets(schema, st, make(map[*types.SimpleType]bool))
						return validateValueAgainstFacets(normalizedValue, st, facets, context)
					} else if errors.Is(err, errCircularReference) {
						sawCycle = true
					} else if firstErr == nil {
						firstErr = err
					}
				}
				if firstErr != nil {
					return firstErr
				}
				if sawCycle {
					return fmt.Errorf("cannot validate default/fixed value for circular union type '%s'", typ.Name().Local)
				}
				return fmt.Errorf("value '%s' does not match any member type of union '%s'", normalizedValue, typ.Name().Local)
			}
		case types.ListVariety:
			itemType := resolveListItemType(schema, st)
			if itemType != nil {
				count := 0
				for item := range types.FieldsXMLWhitespaceSeq(normalizedValue) {
					if err := validateDefaultOrFixedValueResolved(schema, item, itemType, context, visited, policy); err != nil {
						if errors.Is(err, errCircularReference) {
							return fmt.Errorf("cannot validate default/fixed value for circular list item type '%s'", typ.Name().Local)
						}
						return err
					}
					count++
				}
			}
			facets := collectSimpleTypeFacets(schema, st, make(map[*types.SimpleType]bool))
			return validateValueAgainstFacets(normalizedValue, st, facets, context)
		default:
			if types.IsQNameOrNotationType(st) {
				if err := validateQNameContext(normalizedValue, context); err != nil {
					return err
				}
			} else if err := st.Validate(normalizedValue); err != nil {
				return err
			}
			facets := collectSimpleTypeFacets(schema, st, make(map[*types.SimpleType]bool))
			return validateValueAgainstFacets(normalizedValue, st, facets, context)
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
	if st.Restriction != nil && !st.Restriction.Base.IsZero() {
		baseType := schema.TypeDefs[st.Restriction.Base]
		if baseST, ok := baseType.(*types.SimpleType); ok {
			return resolveUnionMemberTypesVisited(schema, baseST, visited)
		}
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
		if enumFacet, ok := facet.(*types.Enumeration); ok && types.IsQNameOrNotationType(baseType) && !isListType(baseType) {
			if err := enumFacet.ValidateLexicalQName(value, baseType, context); err != nil {
				return err
			}
			continue
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

	switch {
	case st.IsBuiltin() && isBuiltinListTypeName(st.QName.Local):
		result = append(result, &types.MinLength{Value: 1})
	case st.ResolvedBase != nil:
		if bt, ok := st.ResolvedBase.(*types.BuiltinType); ok && isBuiltinListTypeName(bt.Name().Local) {
			result = append(result, &types.MinLength{Value: 1})
		}
	case st.Restriction != nil && !st.Restriction.Base.IsZero() &&
		st.Restriction.Base.Namespace == types.XSDNamespace &&
		isBuiltinListTypeName(st.Restriction.Base.Local):
		result = append(result, &types.MinLength{Value: 1})
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

func isBuiltinListTypeName(name string) bool {
	return name == string(types.TypeNameNMTOKENS) ||
		name == string(types.TypeNameIDREFS) ||
		name == string(types.TypeNameENTITIES)
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
	if !types.IsLengthFacet(facet) {
		return false
	}
	if isListType(baseType) {
		return false
	}
	return types.IsQNameOrNotationType(baseType)
}
