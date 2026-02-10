package parser

import (
	"github.com/jacoelho/xsd/internal/facetvalue"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

type orderedFacetConstructor func(string, model.Type) (model.Facet, error)

var orderedFacetConstructors = map[string]orderedFacetConstructor{
	"minInclusive": facetvalue.NewMinInclusive,
	"maxInclusive": facetvalue.NewMaxInclusive,
	"minExclusive": facetvalue.NewMinExclusive,
	"maxExclusive": facetvalue.NewMaxExclusive,
}

type facetParserFunc func(doc *schemaxml.Document, elem schemaxml.NodeID) (model.Facet, error)

var directFacetParsers = map[string]facetParserFunc{
	"pattern":        parsePatternFacet,
	"length":         parseLengthFacet,
	"minLength":      parseMinLengthFacet,
	"maxLength":      parseMaxLengthFacet,
	"totalDigits":    parseTotalDigitsFacet,
	"fractionDigits": parseFractionDigitsFacet,
}
