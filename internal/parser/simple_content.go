package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func parseSimpleContent(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.SimpleContent, error) {
	sc := &types.SimpleContent{}

	if err := validateOptionalID(doc, elem, "simpleContent", schema); err != nil {
		return nil, err
	}

	seenDerivation := false
	seenAnnotation := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if seenDerivation {
				return nil, fmt.Errorf("simpleContent: annotation must appear before restriction or extension")
			}
			if seenAnnotation {
				return nil, fmt.Errorf("simpleContent: at most one annotation is allowed")
			}
			seenAnnotation = true
		case "restriction":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("simpleContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			restriction, baseQName, err := parseSimpleContentRestriction(doc, child, schema)
			if err != nil {
				return nil, err
			}
			sc.Base = baseQName
			sc.Restriction = restriction
		case "extension":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("simpleContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			extension, baseQName, err := parseSimpleContentExtension(doc, child, schema)
			if err != nil {
				return nil, err
			}
			sc.Base = baseQName
			sc.Extension = extension
		default:
			return nil, fmt.Errorf("simpleContent has unexpected child element '%s'", doc.LocalName(child))
		}
	}

	if !seenDerivation {
		return nil, fmt.Errorf("simpleContent must have exactly one derivation child (restriction or extension)")
	}

	return sc, nil
}

func parseSimpleContentRestriction(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.Restriction, types.QName, error) {
	if err := validateOptionalID(doc, elem, "restriction", schema); err != nil {
		return nil, types.QName{}, err
	}
	base := doc.GetAttribute(elem, "base")
	if base == "" {
		return nil, types.QName{}, fmt.Errorf("restriction missing base")
	}
	baseQName, err := resolveQName(doc, base, elem, schema)
	if err != nil {
		return nil, types.QName{}, err
	}
	restriction := &types.Restriction{Base: baseQName}

	err = validateSimpleContentRestrictionOrder(doc, elem)
	if err != nil {
		return nil, baseQName, err
	}

	nestedSimpleType, err := parseSimpleContentNestedType(doc, elem, schema)
	if err != nil {
		return nil, baseQName, err
	}
	restriction.SimpleType = nestedSimpleType

	err = parseFacetsWithAttributes(doc, elem, restriction, nestedSimpleType, schema)
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

func parseSimpleContentExtension(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.Extension, types.QName, error) {
	if err := validateOptionalID(doc, elem, "extension", schema); err != nil {
		return nil, types.QName{}, err
	}
	base := doc.GetAttribute(elem, "base")
	if base == "" {
		return nil, types.QName{}, fmt.Errorf("extension missing base")
	}
	baseQName, err := resolveQName(doc, base, elem, schema)
	if err != nil {
		return nil, types.QName{}, err
	}

	err = validateSimpleContentExtensionChildren(doc, elem)
	if err != nil {
		return nil, baseQName, err
	}

	extension := &types.Extension{Base: baseQName}
	uses, err := parseAttributeUses(doc, doc.Children(elem), schema, "extension")
	if err != nil {
		return nil, baseQName, err
	}
	extension.Attributes = uses.attributes
	extension.AttrGroups = uses.attrGroups
	extension.AnyAttribute = uses.anyAttribute

	return extension, baseQName, nil
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

func parseSimpleContentNestedType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.SimpleType, error) {
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

func validateSimpleContentExtensionChildren(doc *xsdxml.Document, elem xsdxml.NodeID) error {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation", "attribute", "attributeGroup", "anyAttribute":
			continue
		default:
			return fmt.Errorf("simpleContent extension has unexpected child element '%s'", doc.LocalName(child))
		}
	}
	return nil
}
