package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/value"
)

var validNotationAttributes = map[string]bool{
	"name":   true,
	"id":     true,
	"public": true,
	"system": true,
}

func parseTopLevelComponent(doc *Document, elem NodeID, schema *Schema) error {
	localName := doc.LocalName(elem)
	var (
		parseFn   func(*Document, NodeID, *Schema) error
		parseName string
	)

	switch localName {
	case "annotation", "import", "include":
		return nil
	case "element":
		parseFn = parseTopLevelElement
		parseName = "element"
	case "complexType":
		parseFn = parseComplexType
		parseName = "complexType"
	case "simpleType":
		parseFn = parseSimpleType
		parseName = "simpleType"
	case "group":
		parseFn = parseTopLevelGroup
		parseName = "group"
	case "attribute":
		parseFn = parseTopLevelAttribute
		parseName = "attribute"
	case "attributeGroup":
		parseFn = parseTopLevelAttributeGroup
		parseName = "attributeGroup"
	case "notation":
		parseFn = parseTopLevelNotation
		parseName = "notation"
	case "key", "keyref", "unique":
		return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", localName)
	case "redefine":
		return fmt.Errorf("redefine is not supported")
	default:
		return fmt.Errorf("unexpected top-level element '%s'", localName)
	}

	if err := parseFn(doc, elem, schema); err != nil {
		return fmt.Errorf("parse %s: %w", parseName, err)
	}
	return nil
}

// parseTopLevelNotation parses a top-level notation declaration
func parseTopLevelNotation(doc *Document, elem NodeID, schema *Schema) error {
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

	if !model.IsValidNCName(name) {
		return fmt.Errorf("notation name '%s' must be a valid NCName", name)
	}

	if err := validateOptionalID(doc, elem, "notation", schema); err != nil {
		return err
	}

	public := doc.GetAttribute(elem, "public")
	system := doc.GetAttribute(elem, "system")
	if !doc.HasAttribute(elem, "public") && !doc.HasAttribute(elem, "system") {
		return fmt.Errorf("notation must have either 'public' or 'system' attribute")
	}

	hasAnnotation := false
	for _, child := range doc.Children(elem) {
		childName := doc.LocalName(child)
		if doc.NamespaceURI(child) != value.XSDNamespace || childName != "annotation" {
			return fmt.Errorf("notation '%s': unexpected child element '%s'", name, childName)
		}
		if hasAnnotation {
			return fmt.Errorf("notation '%s': at most one annotation is allowed", name)
		}
		hasAnnotation = true
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
