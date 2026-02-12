package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/xmltree"
)

// hasIDAttribute checks if an element has an id attribute (even if empty)
func hasIDAttribute(doc *xmltree.Document, elem xmltree.NodeID) bool {
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "id" && attr.NamespaceURI() == "" {
			return true
		}
	}
	return false
}

func validateOptionalID(doc *xmltree.Document, elem xmltree.NodeID, elementName string, schema *Schema) error {
	if !hasIDAttribute(doc, elem) {
		return nil
	}
	idAttr := doc.GetAttribute(elem, "id")
	return validateIDAttribute(idAttr, elementName, schema)
}

// validateIDAttribute validates that an id attribute is a valid NCName.
// Per XSD spec, id attributes on schema components must be valid NCNames.
// Also registers the id for uniqueness schemacheck.
func validateIDAttribute(id, elementName string, schema *Schema) error {
	if !qname.IsValidNCName(id) {
		return fmt.Errorf("%s element has invalid id attribute '%s': must be a valid NCName", elementName, id)
	}
	if existing, exists := schema.IDAttributes[id]; exists {
		return fmt.Errorf("%s element has duplicate id attribute '%s' (already used by %s)", elementName, id, existing)
	}
	schema.IDAttributes[id] = elementName
	return nil
}
