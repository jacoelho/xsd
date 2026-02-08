package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parsePatternFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	if err := validateOnlyAnnotationChildren(doc, elem, "pattern"); err != nil {
		return nil, err
	}
	value := doc.GetAttribute(elem, "value")
	return &types.Pattern{Value: value}, nil
}

func parseEnumerationFacet(doc *xsdxml.Document, elem xsdxml.NodeID, restriction *types.Restriction, schema *Schema) (types.Facet, error) {
	if err := validateOnlyAnnotationChildren(doc, elem, "enumeration"); err != nil {
		return nil, err
	}
	if !doc.HasAttribute(elem, "value") {
		return nil, fmt.Errorf("enumeration facet missing value attribute")
	}
	value := doc.GetAttribute(elem, "value")
	context := namespaceContextForElement(doc, elem, schema)
	if enum := findEnumerationFacet(restriction.Facets); enum != nil {
		enum.AppendValue(value, context)
		return nil, nil
	}
	enum := types.NewEnumeration([]string{value})
	enum.SetValueContexts([]map[string]string{context})
	return enum, nil
}

func findEnumerationFacet(facets []any) *types.Enumeration {
	for _, facet := range facets {
		if enum, ok := facet.(*types.Enumeration); ok {
			return enum
		}
	}
	return nil
}
