package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

var validNotationAttributes = map[string]bool{
	"name":   true,
	"id":     true,
	"public": true,
	"system": true,
}

func parseComponents(doc *xsdxml.Document, root xsdxml.NodeID, schema *Schema) error {
	for _, child := range doc.Children(root) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation", "import", "include":
		case "element":
			if err := parseTopLevelElement(doc, child, schema); err != nil {
				return fmt.Errorf("parse element: %w", err)
			}
		case "complexType":
			if err := parseComplexType(doc, child, schema); err != nil {
				return fmt.Errorf("parse complexType: %w", err)
			}
		case "simpleType":
			if err := parseSimpleType(doc, child, schema); err != nil {
				return fmt.Errorf("parse simpleType: %w", err)
			}
		case "group":
			if err := parseTopLevelGroup(doc, child, schema); err != nil {
				return fmt.Errorf("parse group: %w", err)
			}
		case "attribute":
			if err := parseTopLevelAttribute(doc, child, schema); err != nil {
				return fmt.Errorf("parse attribute: %w", err)
			}
		case "attributeGroup":
			if err := parseTopLevelAttributeGroup(doc, child, schema); err != nil {
				return fmt.Errorf("parse attributeGroup: %w", err)
			}
		case "notation":
			if err := parseTopLevelNotation(doc, child, schema); err != nil {
				return fmt.Errorf("parse notation: %w", err)
			}
		case "key", "keyref", "unique":
			return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
		case "redefine":
			return fmt.Errorf("redefine is not supported")
		default:
			return fmt.Errorf("unexpected top-level element '%s'", doc.LocalName(child))
		}
	}
	return nil
}

// parseTopLevelNotation parses a top-level notation declaration
func parseTopLevelNotation(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	if err := validateAllowedAttributes(doc, elem, "notation", validNotationAttributes); err != nil {
		return err
	}

	if types.TrimXMLWhitespace(string(doc.DirectTextContentBytes(elem))) != "" {
		return fmt.Errorf("notation must not contain character data")
	}

	name := doc.GetAttribute(elem, "name")
	if name == "" {
		return fmt.Errorf("notation must have a 'name' attribute")
	}

	if !types.IsValidNCName(name) {
		return fmt.Errorf("notation name '%s' must be a valid NCName", name)
	}

	if err := validateOptionalID(doc, elem, "notation", schema); err != nil {
		return err
	}

	public := doc.GetAttribute(elem, "public")
	system := doc.GetAttribute(elem, "system")
	hasPublic := doc.HasAttribute(elem, "public")
	hasSystem := doc.HasAttribute(elem, "system")
	if !hasPublic && !hasSystem {
		return fmt.Errorf("notation must have either 'public' or 'system' attribute")
	}

	hasAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			return fmt.Errorf("notation '%s': unexpected child element '%s'", name, doc.LocalName(child))
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return fmt.Errorf("notation '%s': at most one annotation is allowed", name)
			}
			hasAnnotation = true
		default:
			return fmt.Errorf("notation '%s': unexpected child element '%s'", name, doc.LocalName(child))
		}
	}

	notationQName := types.QName{Local: name, Namespace: schema.TargetNamespace}
	if _, exists := schema.NotationDecls[notationQName]; exists {
		return fmt.Errorf("duplicate notation declaration %s", notationQName.String())
	}

	notation := &types.NotationDecl{
		Name:            notationQName,
		Public:          public,
		System:          system,
		SourceNamespace: schema.TargetNamespace,
	}

	schema.NotationDecls[notationQName] = notation
	schema.addGlobalDecl(GlobalDeclNotation, notationQName)

	return nil
}
