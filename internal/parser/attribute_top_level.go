package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

// parseTopLevelAttribute parses a top-level attribute declaration
func parseTopLevelAttribute(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) error {
	name := model.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
	if name == "" {
		return fmt.Errorf("attribute missing name attribute")
	}

	if doc.HasAttribute(elem, "form") {
		return fmt.Errorf("top-level attribute cannot have 'form' attribute")
	}
	if doc.HasAttribute(elem, "use") {
		return fmt.Errorf("top-level attribute cannot have 'use' attribute")
	}
	if doc.HasAttribute(elem, "ref") {
		return fmt.Errorf("top-level attribute cannot have 'ref' attribute")
	}

	attr, err := parseAttribute(doc, elem, schema, false)
	if err != nil {
		return err
	}

	attrQName := model.QName{
		Local:     name,
		Namespace: schema.TargetNamespace,
	}

	attr.Name = attrQName
	attr.SourceNamespace = schema.TargetNamespace

	if _, exists := schema.AttributeDecls[attrQName]; exists {
		return fmt.Errorf("attribute %s already defined", attrQName)
	}
	schema.AttributeDecls[attrQName] = attr
	schema.addGlobalDecl(GlobalDeclAttribute, attrQName)

	return nil
}
