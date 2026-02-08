package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseListDerivation(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.SimpleType, error) {
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}
	if err := validateOptionalID(doc, elem, "list", schema); err != nil {
		return nil, err
	}

	itemType := doc.GetAttribute(elem, "itemType")
	facetType := &types.SimpleType{}
	facetType.SetWhiteSpace(types.WhiteSpaceCollapse)

	var inlineItemType *types.SimpleType
	var restriction *types.Restriction
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "simpleType":
			if inlineItemType != nil {
				return nil, fmt.Errorf("list cannot have multiple simpleType children")
			}
			var err error
			inlineItemType, err = parseInlineSimpleType(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse inline simpleType in list: %w", err)
			}
		case "restriction":
			if restriction != nil {
				return nil, fmt.Errorf("list cannot have multiple restriction children")
			}
			restriction = &types.Restriction{}
			if err := parseFacets(doc, child, restriction, facetType, schema); err != nil {
				return nil, fmt.Errorf("parse facets in list restriction: %w", err)
			}
		}
	}

	if facetType.WhiteSpaceExplicit() && facetType.WhiteSpace() != types.WhiteSpaceCollapse {
		return nil, fmt.Errorf("list whiteSpace facet must be 'collapse'")
	}

	if itemType != "" && inlineItemType != nil {
		return nil, fmt.Errorf("list cannot have both itemType attribute and inline simpleType child")
	}
	if itemType == "" && inlineItemType == nil {
		return nil, fmt.Errorf("list must have either itemType attribute or inline simpleType child")
	}

	var parsed *types.SimpleType
	var err error
	if inlineItemType != nil {
		list := &types.ListType{
			ItemType:       types.QName{},
			InlineItemType: inlineItemType,
		}
		parsed, err = types.NewListSimpleType(types.QName{}, "", list, restriction)
		if err != nil {
			return nil, fmt.Errorf("simpleType: %w", err)
		}
	} else {
		itemTypeQName, err := resolveQName(doc, itemType, elem, schema)
		if err != nil {
			return nil, err
		}
		list := &types.ListType{ItemType: itemTypeQName}
		parsed, err = types.NewListSimpleType(types.QName{}, "", list, restriction)
		if err != nil {
			return nil, fmt.Errorf("simpleType: %w", err)
		}
	}
	if facetType.WhiteSpaceExplicit() {
		parsed.SetWhiteSpaceExplicit(facetType.WhiteSpace())
	} else {
		parsed.SetWhiteSpace(facetType.WhiteSpace())
	}

	return parsed, nil
}

func parseUnionDerivation(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.SimpleType, error) {
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}
	if err := validateOptionalID(doc, elem, "union", schema); err != nil {
		return nil, err
	}

	memberTypesAttr := doc.GetAttribute(elem, "memberTypes")
	union := &types.UnionType{
		MemberTypes: []types.QName{},
		InlineTypes: []*types.SimpleType{},
	}

	if memberTypesAttr != "" {
		for memberTypeName := range types.FieldsXMLWhitespaceSeq(memberTypesAttr) {
			memberTypeQName, err := resolveQName(doc, memberTypeName, elem, schema)
			if err != nil {
				return nil, fmt.Errorf("resolve member type %s: %w", memberTypeName, err)
			}
			union.MemberTypes = append(union.MemberTypes, memberTypeQName)
		}
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		if doc.LocalName(child) == "simpleType" {
			inlineType, err := parseInlineSimpleType(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse inline simpleType in union: %w", err)
			}
			union.InlineTypes = append(union.InlineTypes, inlineType)
		}
	}

	parsed, err := types.NewUnionSimpleType(types.QName{}, "", union)
	if err != nil {
		return nil, fmt.Errorf("simpleType: %w", err)
	}
	return parsed, nil
}
