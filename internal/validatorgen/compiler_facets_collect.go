package validatorgen

import (
	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

func (c *compiler) collectFacets(st *model.SimpleType) ([]model.Facet, error) {
	if st == nil {
		return nil, nil
	}
	if cached, ok := c.facetsCache[st]; ok {
		return cached, nil
	}

	seen := make(map[*model.SimpleType]bool)
	facets, err := c.collectFacetsRecursive(st, seen)
	if err != nil {
		return nil, err
	}
	c.facetsCache[st] = facets
	return facets, nil
}

func (c *compiler) collectFacetsRecursive(st *model.SimpleType, seen map[*model.SimpleType]bool) ([]model.Facet, error) {
	if st == nil {
		return nil, nil
	}
	if seen[st] {
		return nil, nil
	}
	seen[st] = true
	defer delete(seen, st)

	var result []model.Facet
	if base := c.res.baseType(st); base != nil {
		if baseST, ok := model.AsSimpleType(base); ok {
			baseFacets, err := c.collectFacetsRecursive(baseST, seen)
			if err != nil {
				return nil, err
			}
			result = append(result, baseFacets...)
		}
	}

	if st.IsBuiltin() && builtins.IsBuiltinListTypeName(st.Name().Local) {
		result = append(result, &model.MinLength{Value: 1})
	} else if base := c.res.baseType(st); base != nil {
		if bt := builtinForType(base); bt != nil && builtins.IsBuiltinListTypeName(bt.Name().Local) {
			result = append(result, &model.MinLength{Value: 1})
		}
	}

	if st.Restriction != nil {
		var stepPatterns []*model.Pattern
		for _, f := range st.Restriction.Facets {
			switch facet := f.(type) {
			case model.Facet:
				if patternFacet, ok := facet.(*model.Pattern); ok {
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
			case *model.DeferredFacet:
				base := c.res.baseType(st)
				resolved, err := typeresolve.DefaultDeferredFacetConverter(facet, base)
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
			result = append(result, &model.PatternSet{Patterns: stepPatterns})
		}
	}

	return result, nil
}

func (c *compiler) facetsForType(typ model.Type) ([]model.Facet, error) {
	if st, ok := model.AsSimpleType(typ); ok {
		return c.collectFacets(st)
	}
	if bt, ok := model.AsBuiltinType(typ); ok {
		if builtins.IsBuiltinListTypeName(bt.Name().Local) {
			return []model.Facet{&model.MinLength{Value: 1}}, nil
		}
	}
	return nil, nil
}

func filterFacets(facets []model.Facet, keep func(model.Facet) bool) []model.Facet {
	if len(facets) == 0 {
		return nil
	}
	out := make([]model.Facet, 0, len(facets))
	for _, facet := range facets {
		if keep(facet) {
			out = append(out, facet)
		}
	}
	return out
}
