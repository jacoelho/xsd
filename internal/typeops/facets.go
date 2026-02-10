package typeops

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typefacet"
)

// DeferredFacetConverter converts deferred facets once the base type is known.
type DeferredFacetConverter func(df *model.DeferredFacet, baseType model.Type) (model.Facet, error)

// DefaultDeferredFacetConverter converts deferred range facets using built-in constructors.
func DefaultDeferredFacetConverter(df *model.DeferredFacet, baseType model.Type) (model.Facet, error) {
	if df == nil || baseType == nil {
		return nil, nil
	}
	switch df.FacetName {
	case "minInclusive":
		return typefacet.NewMinInclusive(df.FacetValue, baseType)
	case "maxInclusive":
		return typefacet.NewMaxInclusive(df.FacetValue, baseType)
	case "minExclusive":
		return typefacet.NewMinExclusive(df.FacetValue, baseType)
	case "maxExclusive":
		return typefacet.NewMaxExclusive(df.FacetValue, baseType)
	default:
		return nil, fmt.Errorf("unknown deferred facet type: %s", df.FacetName)
	}
}

// CollectSimpleTypeFacets collects inherited and local facets for a simple type.
func CollectSimpleTypeFacets(schema *parser.Schema, st *model.SimpleType, convert DeferredFacetConverter) ([]model.Facet, error) {
	visited := make(map[*model.SimpleType]bool)
	var visit func(current *model.SimpleType) ([]model.Facet, error)
	visit = func(current *model.SimpleType) ([]model.Facet, error) {
		if current == nil {
			return nil, nil
		}
		if visited[current] {
			return nil, nil
		}
		visited[current] = true
		defer delete(visited, current)

		var result []model.Facet
		if current.ResolvedBase != nil {
			if baseST, ok := current.ResolvedBase.(*model.SimpleType); ok {
				baseFacets, err := visit(baseST)
				if err != nil {
					return nil, err
				}
				result = append(result, baseFacets...)
			}
		} else if current.Restriction != nil && !current.Restriction.Base.IsZero() {
			if base := ResolveSimpleTypeReferenceAllowMissing(schema, current.Restriction.Base); base != nil {
				if baseST, ok := base.(*model.SimpleType); ok {
					baseFacets, err := visit(baseST)
					if err != nil {
						return nil, err
					}
					result = append(result, baseFacets...)
				}
			}
		}

		switch {
		case current.IsBuiltin() && builtins.IsBuiltinListTypeName(current.QName.Local):
			result = append(result, &model.MinLength{Value: 1})
		case current.ResolvedBase != nil:
			if bt, ok := current.ResolvedBase.(*model.BuiltinType); ok && builtins.IsBuiltinListTypeName(bt.Name().Local) {
				result = append(result, &model.MinLength{Value: 1})
			}
		case current.Restriction != nil && !current.Restriction.Base.IsZero() &&
			current.Restriction.Base.Namespace == model.XSDNamespace &&
			builtins.IsBuiltinListTypeName(current.Restriction.Base.Local):
			result = append(result, &model.MinLength{Value: 1})
		}

		if current.Restriction != nil {
			var baseType model.Type
			if current.ResolvedBase != nil {
				baseType = current.ResolvedBase
			} else if !current.Restriction.Base.IsZero() {
				baseType = ResolveSimpleTypeReferenceAllowMissing(schema, current.Restriction.Base)
			}
			restrictionFacets, err := CollectRestrictionFacets(schema, current.Restriction, baseType, convert)
			if err != nil {
				return nil, err
			}
			result = append(result, restrictionFacets...)
		}

		return result, nil
	}

	return visit(st)
}

// CollectRestrictionFacets collects restriction facets and composes patterns when valid.
func CollectRestrictionFacets(schema *parser.Schema, restriction *model.Restriction, baseType model.Type, convert DeferredFacetConverter) ([]model.Facet, error) {
	if restriction == nil || len(restriction.Facets) == 0 {
		return nil, nil
	}
	if convert == nil {
		convert = DefaultDeferredFacetConverter
	}

	var (
		result       []model.Facet
		stepPatterns []*model.Pattern
	)

	for _, facetIface := range restriction.Facets {
		switch facet := facetIface.(type) {
		case *model.Pattern:
			if err := facet.ValidateSyntax(); err != nil {
				return nil, err
			}
			stepPatterns = append(stepPatterns, facet)
		case model.Facet:
			if compilable, ok := facet.(interface{ ValidateSyntax() error }); ok {
				if err := compilable.ValidateSyntax(); err != nil {
					return nil, fmt.Errorf("%s facet: %w", facet.Name(), err)
				}
			}
			result = append(result, facet)
		case *model.DeferredFacet:
			if baseType == nil {
				baseType = ResolveSimpleTypeReferenceAllowMissing(schema, restriction.Base)
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
		result = append(result, &model.PatternSet{Patterns: stepPatterns})
	}

	return result, nil
}
