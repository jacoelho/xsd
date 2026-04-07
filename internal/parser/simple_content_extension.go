package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/value"
)

func parseSimpleContentExtension(doc *Document, elem NodeID, schema *Schema) (*model.Extension, model.QName, error) {
	baseQName, err := parseDerivationBaseQName(doc, elem, schema, "extension")
	if err != nil {
		return nil, model.QName{}, err
	}

	err = validateSimpleContentExtensionChildren(doc, elem)
	if err != nil {
		return nil, baseQName, err
	}

	extension := &model.Extension{Base: baseQName}
	uses, err := parseAttributeUses(doc, doc.Children(elem), schema, "extension")
	if err != nil {
		return nil, baseQName, err
	}
	extension.Attributes = uses.attributes
	extension.AttrGroups = uses.attrGroups
	extension.AnyAttribute = uses.anyAttribute

	return extension, baseQName, nil
}

func validateSimpleContentExtensionChildren(doc *Document, elem NodeID) error {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != value.XSDNamespace {
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
