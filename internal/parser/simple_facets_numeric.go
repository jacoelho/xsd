package parser

import (
	"fmt"
	"strconv"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func parseLengthFacet(doc *xmltree.Document, elem xmltree.NodeID) (model.Facet, error) {
	length, err := parseFacetValueInt(doc, elem, "length")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("length value must be non-negative, got %d", length)
	}
	return &model.Length{Value: length}, nil
}

func parseMinLengthFacet(doc *xmltree.Document, elem xmltree.NodeID) (model.Facet, error) {
	length, err := parseFacetValueInt(doc, elem, "minLength")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("minLength value must be non-negative, got %d", length)
	}
	return &model.MinLength{Value: length}, nil
}

func parseMaxLengthFacet(doc *xmltree.Document, elem xmltree.NodeID) (model.Facet, error) {
	length, err := parseFacetValueInt(doc, elem, "maxLength")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("maxLength value must be non-negative, got %d", length)
	}
	return &model.MaxLength{Value: length}, nil
}

func parseTotalDigitsFacet(doc *xmltree.Document, elem xmltree.NodeID) (model.Facet, error) {
	digits, err := parseFacetValueInt(doc, elem, "totalDigits")
	if err != nil {
		return nil, err
	}
	if digits <= 0 {
		return nil, fmt.Errorf("totalDigits value must be positive, got %d", digits)
	}
	return &model.TotalDigits{Value: digits}, nil
}

func parseFractionDigitsFacet(doc *xmltree.Document, elem xmltree.NodeID) (model.Facet, error) {
	digits, err := parseFacetValueInt(doc, elem, "fractionDigits")
	if err != nil {
		return nil, err
	}
	if digits < 0 {
		return nil, fmt.Errorf("fractionDigits value must be non-negative, got %d", digits)
	}
	return &model.FractionDigits{Value: digits}, nil
}

func parseFacetValueInt(doc *xmltree.Document, elem xmltree.NodeID, facetName string) (int, error) {
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
