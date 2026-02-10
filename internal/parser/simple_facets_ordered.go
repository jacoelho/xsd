package parser

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseOrderedFacet(doc *xsdxml.Document, elem xsdxml.NodeID, restriction *model.Restriction, baseType model.Type, facetName string, constructor orderedFacetConstructor) (model.Facet, error) {
	if err := validateOnlyAnnotationChildren(doc, elem, facetName); err != nil {
		return nil, err
	}
	value := doc.GetAttribute(elem, "value")
	if value == "" {
		return nil, fmt.Errorf("%s facet missing value", facetName)
	}
	if baseType == nil {
		deferFacet(restriction, facetName, value)
		return nil, nil
	}

	facet, err := constructor(value, baseType)
	if err == nil && facet != nil {
		return facet, nil
	}
	if errors.Is(err, model.ErrCannotDeterminePrimitiveType) {
		deferFacet(restriction, facetName, value)
		return nil, nil
	}
	if err == nil {
		return nil, fmt.Errorf("%s facet: %s", facetName, "missing facet")
	}
	return nil, fmt.Errorf("%s facet: %w", facetName, err)
}

func deferFacet(restriction *model.Restriction, facetName, facetValue string) {
	restriction.Facets = append(restriction.Facets, &model.DeferredFacet{
		FacetName:  facetName,
		FacetValue: facetValue,
	})
}
