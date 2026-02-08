package runtimecompile

import (
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) collectFacets(st *types.SimpleType) ([]types.Facet, error) {
	if st == nil {
		return nil, nil
	}
	if cached, ok := c.facetsCache[st]; ok {
		return cached, nil
	}

	seen := make(map[*types.SimpleType]bool)
	facets, err := c.collectFacetsRecursive(st, seen)
	if err != nil {
		return nil, err
	}
	c.facetsCache[st] = facets
	return facets, nil
}

func (c *compiler) collectFacetsRecursive(st *types.SimpleType, seen map[*types.SimpleType]bool) ([]types.Facet, error) {
	if st == nil {
		return nil, nil
	}
	if seen[st] {
		return nil, nil
	}
	seen[st] = true
	defer delete(seen, st)

	var result []types.Facet
	if base := c.res.baseType(st); base != nil {
		if baseST, ok := types.AsSimpleType(base); ok {
			baseFacets, err := c.collectFacetsRecursive(baseST, seen)
			if err != nil {
				return nil, err
			}
			result = append(result, baseFacets...)
		}
	}

	if st.IsBuiltin() && isBuiltinListName(st.Name().Local) {
		result = append(result, &types.MinLength{Value: 1})
	} else if base := c.res.baseType(st); base != nil {
		if bt := builtinForType(base); bt != nil && isBuiltinListName(bt.Name().Local) {
			result = append(result, &types.MinLength{Value: 1})
		}
	}

	if st.Restriction != nil {
		var stepPatterns []*types.Pattern
		for _, f := range st.Restriction.Facets {
			switch facet := f.(type) {
			case types.Facet:
				if patternFacet, ok := facet.(*types.Pattern); ok {
					if err := patternFacet.ValidateSyntax(); err != nil {
						continue
					}
					stepPatterns = append(stepPatterns, patternFacet)
					continue
				}
				if compilable, ok := facet.(interface{ ValidateSyntax() error }); ok {
					if err := compilable.ValidateSyntax(); err != nil {
						continue
					}
				}
				result = append(result, facet)
			case *types.DeferredFacet:
				base := c.res.baseType(st)
				resolved, err := typeops.DefaultDeferredFacetConverter(facet, base)
				if err != nil {
					return nil, err
				}
				if resolved != nil {
					result = append(result, resolved)
				}
			}
		}
		if len(stepPatterns) == 1 {
			result = append(result, stepPatterns[0])
		} else if len(stepPatterns) > 1 {
			result = append(result, &types.PatternSet{Patterns: stepPatterns})
		}
	}

	return result, nil
}

func (c *compiler) facetsForType(typ types.Type) ([]types.Facet, error) {
	if st, ok := types.AsSimpleType(typ); ok {
		return c.collectFacets(st)
	}
	if bt, ok := types.AsBuiltinType(typ); ok {
		if isBuiltinListName(bt.Name().Local) {
			return []types.Facet{&types.MinLength{Value: 1}}, nil
		}
	}
	return nil, nil
}

func filterFacets(facets []types.Facet, keep func(types.Facet) bool) []types.Facet {
	if len(facets) == 0 {
		return nil
	}
	out := make([]types.Facet, 0, len(facets))
	for _, facet := range facets {
		if keep(facet) {
			out = append(out, facet)
		}
	}
	return out
}
