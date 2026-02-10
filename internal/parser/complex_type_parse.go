package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

// parseComplexType parses a top-level complexType definition.
func parseComplexType(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) error {
	name := model.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
	if name == "" {
		return fmt.Errorf("complexType missing name attribute")
	}

	if err := validateOptionalID(doc, elem, "complexType", schema); err != nil {
		return err
	}

	ct, err := parseInlineComplexType(doc, elem, schema)
	if err != nil {
		return err
	}

	ct.QName = model.QName{Namespace: schema.TargetNamespace, Local: name}
	ct.SourceNamespace = schema.TargetNamespace

	if _, exists := schema.TypeDefs[ct.QName]; exists {
		return fmt.Errorf("duplicate type definition: '%s'", ct.QName)
	}

	schema.TypeDefs[ct.QName] = ct
	schema.addGlobalDecl(GlobalDeclType, ct.QName)
	return nil
}
