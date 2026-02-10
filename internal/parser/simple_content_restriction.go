package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseSimpleContentRestriction(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*model.Restriction, model.QName, error) {
	if err := validateOptionalID(doc, elem, "restriction", schema); err != nil {
		return nil, model.QName{}, err
	}
	base := doc.GetAttribute(elem, "base")
	if base == "" {
		return nil, model.QName{}, fmt.Errorf("restriction missing base")
	}
	baseQName, err := resolveQNameWithPolicy(doc, base, elem, schema, useDefaultNamespace)
	if err != nil {
		return nil, model.QName{}, err
	}
	restriction := &model.Restriction{Base: baseQName}

	err = validateSimpleContentRestrictionOrder(doc, elem)
	if err != nil {
		return nil, baseQName, err
	}

	nestedSimpleType, err := parseSimpleContentNestedType(doc, elem, schema)
	if err != nil {
		return nil, baseQName, err
	}
	restriction.SimpleType = nestedSimpleType

	err = parseFacetsWithPolicy(doc, elem, restriction, nestedSimpleType, schema, facetAttributesAllowed)
	if err != nil {
		return nil, baseQName, fmt.Errorf("parse facets: %w", err)
	}

	uses, err := parseAttributeUses(doc, doc.Children(elem), schema, "restriction")
	if err != nil {
		return nil, baseQName, err
	}
	restriction.Attributes = uses.attributes
	restriction.AttrGroups = uses.attrGroups
	restriction.AnyAttribute = uses.anyAttribute

	return restriction, baseQName, nil
}

func validateSimpleContentRestrictionOrder(doc *xsdxml.Document, elem xsdxml.NodeID) error {
	seenSimpleType := false
	seenAttributeLike := false
	seenFacet := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			continue
		case "simpleType":
			if seenSimpleType || seenFacet || seenAttributeLike {
				return fmt.Errorf("simpleContent restriction: simpleType must appear before facets and attributes")
			}
			seenSimpleType = true
		case "attribute", "attributeGroup", "anyAttribute":
			seenAttributeLike = true
		default:
			if validChildElementNames[childSetSimpleContentFacet][doc.LocalName(child)] {
				if seenAttributeLike {
					return fmt.Errorf("simpleContent restriction: facets must appear before attributes")
				}
				seenFacet = true
			}
		}
	}

	return nil
}

func parseSimpleContentNestedType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*model.SimpleType, error) {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) == xsdxml.XSDNamespace && doc.LocalName(child) == "simpleType" {
			nestedSimpleType, err := parseInlineSimpleType(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse nested simpleType: %w", err)
			}
			return nestedSimpleType, nil
		}
	}
	return nil, nil
}
