package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/xsdxml"
)

func validateElementAttributes(doc *xsdxml.Document, elem xsdxml.NodeID, validAttributes map[string]bool, context string) error {
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() == xsdxml.XSDNamespace {
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
