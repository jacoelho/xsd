package parser

import (
	"fmt"
	"strconv"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseLengthFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	length, err := parseFacetValueInt(doc, elem, "length")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("length value must be non-negative, got %d", length)
	}
	return &types.Length{Value: length}, nil
}

func parseMinLengthFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	length, err := parseFacetValueInt(doc, elem, "minLength")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("minLength value must be non-negative, got %d", length)
	}
	return &types.MinLength{Value: length}, nil
}

func parseMaxLengthFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	length, err := parseFacetValueInt(doc, elem, "maxLength")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("maxLength value must be non-negative, got %d", length)
	}
	return &types.MaxLength{Value: length}, nil
}

func parseTotalDigitsFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	digits, err := parseFacetValueInt(doc, elem, "totalDigits")
	if err != nil {
		return nil, err
	}
	if digits <= 0 {
		return nil, fmt.Errorf("totalDigits value must be positive, got %d", digits)
	}
	return &types.TotalDigits{Value: digits}, nil
}

func parseFractionDigitsFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	digits, err := parseFacetValueInt(doc, elem, "fractionDigits")
	if err != nil {
		return nil, err
	}
	if digits < 0 {
		return nil, fmt.Errorf("fractionDigits value must be non-negative, got %d", digits)
	}
	return &types.FractionDigits{Value: digits}, nil
}

func parseFacetValueInt(doc *xsdxml.Document, elem xsdxml.NodeID, facetName string) (int, error) {
	if err := validateOnlyAnnotationChildren(doc, elem, facetName); err != nil {
		return 0, err
	}
	value := doc.GetAttribute(elem, "value")
	if value == "" {
		return 0, fmt.Errorf("%s facet missing value", facetName)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value: %w", facetName, err)
	}
	return parsed, nil
}
