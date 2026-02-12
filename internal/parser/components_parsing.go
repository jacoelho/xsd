package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/xmltree"
)

var validNotationAttributes = map[string]bool{
	"name":   true,
	"id":     true,
	"public": true,
	"system": true,
}

func parseTopLevelComponent(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) error {
	switch doc.LocalName(elem) {
	case "annotation", "import", "include":
		return nil
	case "element":
		if err := parseTopLevelElement(doc, elem, schema); err != nil {
			return fmt.Errorf("parse element: %w", err)
		}
		return nil
	case "complexType":
		if err := parseComplexType(doc, elem, schema); err != nil {
			return fmt.Errorf("parse complexType: %w", err)
		}
		return nil
	case "simpleType":
		if err := parseSimpleType(doc, elem, schema); err != nil {
			return fmt.Errorf("parse simpleType: %w", err)
		}
		return nil
	case "group":
		if err := parseTopLevelGroup(doc, elem, schema); err != nil {
			return fmt.Errorf("parse group: %w", err)
		}
		return nil
	case "attribute":
		if err := parseTopLevelAttribute(doc, elem, schema); err != nil {
			return fmt.Errorf("parse attribute: %w", err)
		}
		return nil
	case "attributeGroup":
		if err := parseTopLevelAttributeGroup(doc, elem, schema); err != nil {
			return fmt.Errorf("parse attributeGroup: %w", err)
		}
		return nil
	case "notation":
		if err := parseTopLevelNotation(doc, elem, schema); err != nil {
			return fmt.Errorf("parse notation: %w", err)
		}
		return nil
	case "key", "keyref", "unique":
		return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(elem))
	case "redefine":
		return fmt.Errorf("redefine is not supported")
	default:
		return fmt.Errorf("unexpected top-level element '%s'", doc.LocalName(elem))
	}
}

// parseTopLevelNotation parses a top-level notation declaration
func parseTopLevelNotation(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) error {
	if err := validateAllowedAttributes(doc, elem, "notation", validNotationAttributes); err != nil {
		return err
	}

	if model.TrimXMLWhitespace(string(doc.DirectTextContentBytes(elem))) != "" {
		return fmt.Errorf("notation must not contain character data")
	}

	name := doc.GetAttribute(elem, "name")
	if name == "" {
		return fmt.Errorf("notation must have a 'name' attribute")
	}

	if !qname.IsValidNCName(name) {
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
		if doc.NamespaceURI(child) != xmltree.XSDNamespace {
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

	notationQName := model.QName{Local: name, Namespace: schema.TargetNamespace}
	if _, exists := schema.NotationDecls[notationQName]; exists {
		return fmt.Errorf("duplicate notation declaration %s", notationQName.String())
	}

	notation := &model.NotationDecl{
		Name:            notationQName,
		Public:          public,
		System:          system,
		SourceNamespace: schema.TargetNamespace,
	}

	schema.NotationDecls[notationQName] = notation
	schema.addGlobalDecl(GlobalDeclNotation, notationQName)

	return nil
}
