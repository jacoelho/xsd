package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

type attributeUses struct {
	anyAttribute *model.AnyAttribute
	attributes   []*model.AttributeDecl
	attrGroups   []model.QName
}

func parseAttributeUses(doc *xmltree.Document, children []xmltree.NodeID, schema *Schema, context string) (attributeUses, error) {
	uses := attributeUses{
		attributes: []*model.AttributeDecl{},
		attrGroups: []model.QName{},
	}
	hasAnyAttribute := false

	for _, child := range children {
		if doc.NamespaceURI(child) != xmltree.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			continue
		case "attribute":
			if hasAnyAttribute {
				return uses, fmt.Errorf("%s: anyAttribute must appear after all attributes", context)
			}
			attr, err := parseAttribute(doc, child, schema, true)
			if err != nil {
				return uses, fmt.Errorf("parse attribute in %s: %w", context, err)
			}
			uses.attributes = append(uses.attributes, attr)
		case "attributeGroup":
			if hasAnyAttribute {
				return uses, fmt.Errorf("%s: anyAttribute must appear after all attributes", context)
			}
			refQName, err := parseAttributeGroupRefQName(doc, child, schema)
			if err != nil {
				return uses, err
			}
			uses.attrGroups = append(uses.attrGroups, refQName)
		case "anyAttribute":
			if hasAnyAttribute {
				return uses, fmt.Errorf("%s: at most one anyAttribute is allowed", context)
			}
			hasAnyAttribute = true
			anyAttr, err := parseAnyAttribute(doc, child, schema)
			if err != nil {
				return uses, fmt.Errorf("parse anyAttribute in %s: %w", context, err)
			}
			uses.anyAttribute = anyAttr
		}
	}

	return uses, nil
}
