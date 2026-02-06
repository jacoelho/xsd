package typeops

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// DeferredFacetConverter converts deferred facets once the base type is known.
type DeferredFacetConverter func(df *types.DeferredFacet, baseType types.Type) (types.Facet, error)

// DefaultDeferredFacetConverter converts deferred range facets using built-in constructors.
func DefaultDeferredFacetConverter(df *types.DeferredFacet, baseType types.Type) (types.Facet, error) {
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

// CollectSimpleTypeFacets collects inherited and local facets for a simple type.
func CollectSimpleTypeFacets(schema *parser.Schema, st *types.SimpleType, convert DeferredFacetConverter) ([]types.Facet, error) {
	return collectSimpleTypeFacetsVisited(schema, st, make(map[*types.SimpleType]bool), convert)
}

func collectSimpleTypeFacetsVisited(schema *parser.Schema, st *types.SimpleType, visited map[*types.SimpleType]bool, convert DeferredFacetConverter) ([]types.Facet, error) {
	if st == nil {
		return nil, nil
	}
	if visited[st] {
		return nil, nil
	}
	visited[st] = true
	defer delete(visited, st)

	var result []types.Facet
	if st.ResolvedBase != nil {
		if baseST, ok := st.ResolvedBase.(*types.SimpleType); ok {
			baseFacets, err := collectSimpleTypeFacetsVisited(schema, baseST, visited, convert)
			if err != nil {
				return nil, err
			}
			result = append(result, baseFacets...)
		}
	} else if st.Restriction != nil && !st.Restriction.Base.IsZero() {
		if base := ResolveSimpleTypeReference(schema, st.Restriction.Base); base != nil {
			if baseST, ok := base.(*types.SimpleType); ok {
				baseFacets, err := collectSimpleTypeFacetsVisited(schema, baseST, visited, convert)
				if err != nil {
					return nil, err
				}
				result = append(result, baseFacets...)
			}
		}
	}

	switch {
	case st.IsBuiltin() && types.IsBuiltinListTypeName(st.QName.Local):
		result = append(result, &types.MinLength{Value: 1})
	case st.ResolvedBase != nil:
		if bt, ok := st.ResolvedBase.(*types.BuiltinType); ok && types.IsBuiltinListTypeName(bt.Name().Local) {
			result = append(result, &types.MinLength{Value: 1})
		}
	case st.Restriction != nil && !st.Restriction.Base.IsZero() &&
		st.Restriction.Base.Namespace == types.XSDNamespace &&
		types.IsBuiltinListTypeName(st.Restriction.Base.Local):
		result = append(result, &types.MinLength{Value: 1})
	}

	if st.Restriction != nil {
		var baseType types.Type
		if st.ResolvedBase != nil {
			baseType = st.ResolvedBase
		} else if !st.Restriction.Base.IsZero() {
			baseType = ResolveSimpleTypeReference(schema, st.Restriction.Base)
		}
		restrictionFacets, err := CollectRestrictionFacets(schema, st.Restriction, baseType, convert)
		if err != nil {
			return nil, err
		}
		result = append(result, restrictionFacets...)
	}

	return result, nil
}

// CollectRestrictionFacets collects restriction facets and composes patterns when valid.
func CollectRestrictionFacets(schema *parser.Schema, restriction *types.Restriction, baseType types.Type, convert DeferredFacetConverter) ([]types.Facet, error) {
	if restriction == nil || len(restriction.Facets) == 0 {
		return nil, nil
	}
	if convert == nil {
		convert = DefaultDeferredFacetConverter
	}

	var (
		result       []types.Facet
		stepPatterns []*types.Pattern
	)

	for _, facetIface := range restriction.Facets {
		switch facet := facetIface.(type) {
		case *types.Pattern:
			if err := facet.ValidateSyntax(); err != nil {
				return nil, err
			}
			stepPatterns = append(stepPatterns, facet)
		case types.Facet:
			if compilable, ok := facet.(interface{ ValidateSyntax() error }); ok {
				if err := compilable.ValidateSyntax(); err != nil {
					return nil, fmt.Errorf("%s facet: %w", facet.Name(), err)
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
			resolved, err := convert(facet, baseType)
			if err != nil {
				return nil, err
			}
			if resolved == nil {
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

	return result, nil
}
