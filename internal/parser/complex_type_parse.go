package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

// parseComplexType parses a top-level complexType definition.
func parseComplexType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	name := getNameAttr(doc, elem)
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

	ct.QName = types.QName{Namespace: schema.TargetNamespace, Local: name}
	ct.SourceNamespace = schema.TargetNamespace

	if _, exists := schema.TypeDefs[ct.QName]; exists {
		return fmt.Errorf("duplicate type definition: '%s'", ct.QName)
	}

	schema.TypeDefs[ct.QName] = ct
	schema.addGlobalDecl(GlobalDeclType, ct.QName)
	return nil
}
