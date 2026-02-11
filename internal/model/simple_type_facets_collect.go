package model

import "fmt"

// FacetTypeResolver resolves simple type QNames for facet inheritance.
type FacetTypeResolver func(QName) Type

// DeferredFacetConverter converts deferred facets once base type is known.
type DeferredFacetConverter func(df *DeferredFacet, baseType Type) (Facet, error)

// CollectSimpleTypeFacetsWithResolver collects inherited and local simple-type facets.
func CollectSimpleTypeFacetsWithResolver(st *SimpleType, resolve FacetTypeResolver, convert DeferredFacetConverter) ([]Facet, error) {
	visited := make(map[*SimpleType]bool)
	var visit func(current *SimpleType) ([]Facet, error)
	visit = func(current *SimpleType) ([]Facet, error) {
		if current == nil {
			return nil, nil
		}
		if visited[current] {
			return nil, nil
		}
		visited[current] = true
		defer delete(visited, current)

		var result []Facet
		baseType := current.ResolvedBase
		if baseType == nil && current.Restriction != nil && !current.Restriction.Base.IsZero() {
			baseType = resolveFacetType(current.Restriction.Base, resolve)
		}
		if baseST, ok := AsSimpleType(baseType); ok {
			baseFacets, err := visit(baseST)
			if err != nil {
				return nil, err
			}
			result = append(result, baseFacets...)
		}

		if needsBuiltinListMinLength(current) {
			result = append(result, &MinLength{Value: 1})
		}

		if current.Restriction != nil {
			restrictionFacets, err := CollectRestrictionFacetsWithResolver(current.Restriction, baseType, resolve, convert)
			if err != nil {
				return nil, err
			}
			result = append(result, restrictionFacets...)
		}

		return result, nil
	}

	return visit(st)
}

// CollectRestrictionFacetsWithResolver collects restriction facets and composes same-step patterns.
func CollectRestrictionFacetsWithResolver(restriction *Restriction, baseType Type, resolve FacetTypeResolver, convert DeferredFacetConverter) ([]Facet, error) {
	if restriction == nil || len(restriction.Facets) == 0 {
		return nil, nil
	}

	var (
		result       []Facet
		stepPatterns []*Pattern
	)

	for _, facetIface := range restriction.Facets {
		switch facet := facetIface.(type) {
		case *Pattern:
			if err := facet.ValidateSyntax(); err != nil {
				return nil, err
			}
			stepPatterns = append(stepPatterns, facet)
		case Facet:
			if compilable, ok := facet.(interface{ ValidateSyntax() error }); ok {
				if err := compilable.ValidateSyntax(); err != nil {
					return nil, fmt.Errorf("%s facet: %w", facet.Name(), err)
				}
			}
			result = append(result, facet)
		case *DeferredFacet:
			if convert == nil {
				continue
			}
			if baseType == nil {
				baseType = resolveFacetType(restriction.Base, resolve)
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
		result = append(result, &PatternSet{Patterns: stepPatterns})
	}
	return result, nil
}

func resolveFacetType(qname QName, resolve FacetTypeResolver) Type {
	if qname.IsZero() {
		return nil
	}
	if builtin := GetBuiltinNS(qname.Namespace, qname.Local); builtin != nil {
		return builtin
	}
	if resolve == nil {
		return nil
	}
	return resolve(qname)
}
