package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func parsePatternFacet(doc *schemaxml.Document, elem schemaxml.NodeID) (model.Facet, error) {
	if err := validateOnlyAnnotationChildren(doc, elem, "pattern"); err != nil {
		return nil, err
	}
	value := doc.GetAttribute(elem, "value")
	return &model.Pattern{Value: value}, nil
}

func parseEnumerationFacet(doc *schemaxml.Document, elem schemaxml.NodeID, restriction *model.Restriction, schema *Schema) (model.Facet, error) {
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
	enum := model.NewEnumeration([]string{value})
	enum.SetValueContexts([]map[string]string{context})
	return enum, nil
}

func findEnumerationFacet(facets []any) *model.Enumeration {
	for _, facet := range facets {
		if enum, ok := facet.(*model.Enumeration); ok {
			return enum
		}
	}
	return nil
}
