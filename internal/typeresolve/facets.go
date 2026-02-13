package typeresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/facetvalue"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// DefaultDeferredFacetConverter converts deferred range facets using built-in constructors.
func DefaultDeferredFacetConverter(df *model.DeferredFacet, baseType model.Type) (model.Facet, error) {
	if df == nil || baseType == nil {
		return nil, nil
	}
	switch df.FacetName {
	case "minInclusive":
		return facetvalue.NewMinInclusive(df.FacetValue, baseType)
	case "maxInclusive":
		return facetvalue.NewMaxInclusive(df.FacetValue, baseType)
	case "minExclusive":
		return facetvalue.NewMinExclusive(df.FacetValue, baseType)
	case "maxExclusive":
		return facetvalue.NewMaxExclusive(df.FacetValue, baseType)
	default:
		return nil, fmt.Errorf("unknown deferred facet type: %s", df.FacetName)
	}
}

// CollectSimpleTypeFacets collects inherited and local facets for a simple type.
func CollectSimpleTypeFacets(schema *parser.Schema, st *model.SimpleType, convert model.DeferredFacetConverter) ([]model.Facet, error) {
	if convert == nil {
		convert = DefaultDeferredFacetConverter
	}
	return model.CollectSimpleTypeFacetsWithResolver(
		st,
		func(name model.QName) model.Type {
			return ResolveSimpleTypeReferenceAllowMissing(schema, name)
		},
		convert,
	)
}

// CollectRestrictionFacets collects restriction facets and composes patterns when valid.
func CollectRestrictionFacets(schema *parser.Schema, restriction *model.Restriction, baseType model.Type, convert model.DeferredFacetConverter) ([]model.Facet, error) {
	if convert == nil {
		convert = DefaultDeferredFacetConverter
	}
	return model.CollectRestrictionFacetsWithResolver(
		restriction,
		baseType,
		func(name model.QName) model.Type {
			return ResolveSimpleTypeReferenceAllowMissing(schema, name)
		},
		convert,
	)
}
