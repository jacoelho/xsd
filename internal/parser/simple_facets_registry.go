package parser

import (
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

type orderedFacetConstructor func(string, types.Type) (types.Facet, error)

var orderedFacetConstructors = map[string]orderedFacetConstructor{
	"minInclusive": types.NewMinInclusive,
	"maxInclusive": types.NewMaxInclusive,
	"minExclusive": types.NewMinExclusive,
	"maxExclusive": types.NewMaxExclusive,
}

type facetParserFunc func(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error)

var directFacetParsers = map[string]facetParserFunc{
	"pattern":        parsePatternFacet,
	"length":         parseLengthFacet,
	"minLength":      parseMinLengthFacet,
	"maxLength":      parseMaxLengthFacet,
	"totalDigits":    parseTotalDigitsFacet,
	"fractionDigits": parseFractionDigitsFacet,
}
