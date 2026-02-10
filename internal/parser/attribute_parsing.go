package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

var validAttributeAttributes = map[string]bool{
	"name":    true,
	"ref":     true,
	"type":    true,
	"use":     true,
	"default": true,
	"fixed":   true,
	"form":    true,
	"id":      true,
}

func parseAttribute(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, local bool) (*model.AttributeDecl, error) {
	if err := validateOptionalID(doc, elem, "attribute", schema); err != nil {
		return nil, err
	}
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}

	ref := doc.GetAttribute(elem, "ref")
	nameAttr := doc.GetAttribute(elem, "name")
	if ref != "" && nameAttr != "" {
		return nil, fmt.Errorf("attribute cannot have both 'name' and 'ref' attributes")
	}

	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() == schemaxml.XSDNamespace {
			return nil, fmt.Errorf("attribute: attribute '%s' must be unprefixed", attr.LocalName())
		}
		if attr.NamespaceURI() == "" && !validAttributeAttributes[attr.LocalName()] {
			return nil, fmt.Errorf("invalid attribute '%s' on <attribute> element", attr.LocalName())
		}
	}

	if doc.HasAttribute(elem, "default") && doc.HasAttribute(elem, "fixed") {
		return nil, fmt.Errorf("attribute cannot have both 'default' and 'fixed' attributes")
	}

	if ref != "" {
		return parseAttributeReference(doc, elem, schema, ref)
	}
	return parseLocalAttribute(doc, elem, schema, local)
}

func parseAttributeUse(doc *schemaxml.Document, elem schemaxml.NodeID) (model.AttributeUse, error) {
	if doc.HasAttribute(elem, "use") {
		useAttr := model.ApplyWhiteSpace(doc.GetAttribute(elem, "use"), model.WhiteSpaceCollapse)
		switch useAttr {
		case "optional":
			return model.Optional, nil
		case "required":
			return model.Required, nil
		case "prohibited":
			return model.Prohibited, nil
		default:
			return model.Optional, fmt.Errorf("invalid use attribute value '%s': must be 'optional', 'prohibited', or 'required'", useAttr)
		}
	}

	return model.Optional, nil
}
