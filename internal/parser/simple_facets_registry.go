package parser

import (
	"github.com/jacoelho/xsd/internal/facetvalue"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

type orderedFacetConstructor func(string, model.Type) (model.Facet, error)

var orderedFacetConstructors = map[string]orderedFacetConstructor{
	"minInclusive": facetvalue.NewMinInclusive,
	"maxInclusive": facetvalue.NewMaxInclusive,
	"minExclusive": facetvalue.NewMinExclusive,
	"maxExclusive": facetvalue.NewMaxExclusive,
}

type facetParserFunc func(doc *xmltree.Document, elem xmltree.NodeID) (model.Facet, error)

var directFacetParsers = map[string]facetParserFunc{
	"pattern":        parsePatternFacet,
	"length":         parseLengthFacet,
	"minLength":      parseMinLengthFacet,
	"maxLength":      parseMaxLengthFacet,
	"totalDigits":    parseTotalDigitsFacet,
	"fractionDigits": parseFractionDigitsFacet,
}
