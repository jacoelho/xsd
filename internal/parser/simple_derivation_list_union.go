package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func parseListDerivation(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) (*model.SimpleType, error) {
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}
	if err := validateOptionalID(doc, elem, "list", schema); err != nil {
		return nil, err
	}

	itemType := doc.GetAttribute(elem, "itemType")
	facetType := &model.SimpleType{}
	facetType.SetWhiteSpace(model.WhiteSpaceCollapse)

	var inlineItemType *model.SimpleType
	var restriction *model.Restriction
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xmltree.XSDNamespace {
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
			restriction = &model.Restriction{}
			if err := parseFacetsWithPolicy(doc, child, restriction, facetType, schema, facetAttributesDisallowed); err != nil {
				return nil, fmt.Errorf("parse facets in list restriction: %w", err)
			}
		}
	}

	if facetType.WhiteSpaceExplicit() && facetType.WhiteSpace() != model.WhiteSpaceCollapse {
		return nil, fmt.Errorf("list whiteSpace facet must be 'collapse'")
	}

	if itemType != "" && inlineItemType != nil {
		return nil, fmt.Errorf("list cannot have both itemType attribute and inline simpleType child")
	}
	if itemType == "" && inlineItemType == nil {
		return nil, fmt.Errorf("list must have either itemType attribute or inline simpleType child")
	}

	var parsed *model.SimpleType
	var err error
	if inlineItemType != nil {
		list := &model.ListType{
			ItemType:       model.QName{},
			InlineItemType: inlineItemType,
		}
		parsed, err = model.NewListSimpleType(model.QName{}, "", list, restriction)
		if err != nil {
			return nil, fmt.Errorf("simpleType: %w", err)
		}
	} else {
		itemTypeQName, err := resolveQNameWithPolicy(doc, itemType, elem, schema, useDefaultNamespace)
		if err != nil {
			return nil, err
		}
		list := &model.ListType{ItemType: itemTypeQName}
		parsed, err = model.NewListSimpleType(model.QName{}, "", list, restriction)
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

func parseUnionDerivation(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) (*model.SimpleType, error) {
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}
	if err := validateOptionalID(doc, elem, "union", schema); err != nil {
		return nil, err
	}

	memberTypesAttr := doc.GetAttribute(elem, "memberTypes")
	union := &model.UnionType{
		MemberTypes: []model.QName{},
		InlineTypes: []*model.SimpleType{},
	}

	if memberTypesAttr != "" {
		for memberTypeName := range model.FieldsXMLWhitespaceSeq(memberTypesAttr) {
			memberTypeQName, err := resolveQNameWithPolicy(doc, memberTypeName, elem, schema, useDefaultNamespace)
			if err != nil {
				return nil, fmt.Errorf("resolve member type %s: %w", memberTypeName, err)
			}
			union.MemberTypes = append(union.MemberTypes, memberTypeQName)
		}
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xmltree.XSDNamespace {
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

	parsed, err := model.NewUnionSimpleType(model.QName{}, "", union)
	if err != nil {
		return nil, fmt.Errorf("simpleType: %w", err)
	}
	return parsed, nil
}
