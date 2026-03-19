package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/xmlnames"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func validateElementAttributes(doc *xmltree.Document, elem xmltree.NodeID, validAttributes map[string]bool, context string) error {
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() == xmlnames.XSDNamespace {
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
