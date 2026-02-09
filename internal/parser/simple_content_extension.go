package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseSimpleContentExtension(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.Extension, types.QName, error) {
	if err := validateOptionalID(doc, elem, "extension", schema); err != nil {
		return nil, types.QName{}, err
	}
	base := doc.GetAttribute(elem, "base")
	if base == "" {
		return nil, types.QName{}, fmt.Errorf("extension missing base")
	}
	baseQName, err := resolveQNameWithPolicy(doc, base, elem, schema, useDefaultNamespace)
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
