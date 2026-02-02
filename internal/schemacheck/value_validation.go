package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
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
		baseType := resolveSimpleContentBaseTypeFromContent(schema, sc)
		if baseType == nil {
			return nil
		}
		if sc.Restriction != nil {
			normalized := types.NormalizeWhiteSpace(value, baseType)
			facets := collectRestrictionFacets(schema, sc.Restriction, baseType)
			if err := validateValueAgainstFacets(normalized, baseType, facets, context); err != nil {
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
		if _, err := types.ParseQNameValue(normalized, context); err != nil {
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
		memberTypes := resolveUnionMemberTypesForValidation(schema, st, make(map[*types.SimpleType]bool))
		if len(memberTypes) == 0 {
			return fmt.Errorf("union has no member types")
		}
		for _, member := range memberTypes {
			if err := validateValueAgainstTypeWithFacets(schema, normalized, member, context, visited); err == nil {
				facets := collectSimpleTypeFacets(schema, st, make(map[*types.SimpleType]bool))
				return validateValueAgainstFacets(normalized, st, facets, context)
			}
		}
		return fmt.Errorf("value %q does not match any member type of union", normalized)
	case types.ListVariety:
		itemType := resolveListItemTypeForValidation(schema, st)
		if itemType == nil {
			return nil
		}
		count := 0
		for item := range types.FieldsXMLWhitespaceSeq(normalized) {
			if err := validateValueAgainstTypeWithFacets(schema, item, itemType, context, visited); err != nil {
				return err
			}
			count++
		}
		facets := collectSimpleTypeFacets(schema, st, make(map[*types.SimpleType]bool))
		return validateValueAgainstFacets(normalized, st, facets, context)
	default:
		if !types.IsQNameOrNotationType(st) {
			if err := st.Validate(normalized); err != nil {
				return err
			}
		}
		facets := collectSimpleTypeFacets(schema, st, make(map[*types.SimpleType]bool))
		return validateValueAgainstFacets(normalized, st, facets, context)
	}
}

func resolveSimpleContentBaseTypeFromContent(schema *parser.Schema, sc *types.SimpleContent) types.Type {
	if sc == nil {
		return nil
	}
	var baseQName types.QName
	if sc.Extension != nil {
		baseQName = sc.Extension.Base
	} else if sc.Restriction != nil {
		baseQName = sc.Restriction.Base
	}
	baseType, _ := resolveSimpleContentBaseType(schema, baseQName)
	return baseType
}

func resolveUnionMemberTypesForValidation(schema *parser.Schema, st *types.SimpleType, visited map[*types.SimpleType]bool) []types.Type {
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
			if member := ResolveSimpleTypeReference(schema, memberQName); member != nil {
				memberTypes = append(memberTypes, member)
			}
		}
		return memberTypes
	}
	if st.Restriction != nil && !st.Restriction.Base.IsZero() {
		baseType := schema.TypeDefs[st.Restriction.Base]
		if baseST, ok := baseType.(*types.SimpleType); ok {
			return resolveUnionMemberTypesForValidation(schema, baseST, visited)
		}
	}
	return nil
}

func resolveListItemTypeForValidation(schema *parser.Schema, st *types.SimpleType) types.Type {
	if st == nil {
		return nil
	}
	if st.ItemType != nil {
		return st.ItemType
	}
	if st.List != nil && st.List.InlineItemType != nil {
		return st.List.InlineItemType
	}
	if st.List != nil && !st.List.ItemType.IsZero() {
		return ResolveSimpleTypeReference(schema, st.List.ItemType)
	}
	if itemType, ok := types.ListItemType(st); ok {
		return itemType
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
		if base := ResolveSimpleTypeReference(schema, st.Restriction.Base); base != nil {
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
			baseType = ResolveSimpleTypeReference(schema, st.Restriction.Base)
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
				baseType = ResolveSimpleTypeReference(schema, restriction.Base)
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
