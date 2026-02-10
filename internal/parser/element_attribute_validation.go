package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schemaxml"
)

func validateElementAttributes(doc *schemaxml.Document, elem schemaxml.NodeID, validAttributes map[string]bool, context string) error {
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() == schemaxml.XSDNamespace {
			return fmt.Errorf("%s: attribute '%s' must be unprefixed", context, attr.LocalName())
		}
		if attr.NamespaceURI() != "" {
			continue
		}
		if !validAttributes[attr.LocalName()] {
			return fmt.Errorf("invalid attribute '%s' on %s", attr.LocalName(), context)
		}
	}
	return nil
}
