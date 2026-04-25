package schemaast

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/value"
)

func validateElementAttributes(doc *Document, elem NodeID, validAttributes map[string]bool, context string) error {
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() == value.XSDNamespace {
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

func validateAllowedAttributes(doc *Document, elem NodeID, elementName string, allowed map[string]bool) error {
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() != "" {
			if attr.NamespaceURI() == XSDNamespace {
				return fmt.Errorf("%s: attribute '%s' must be unprefixed", elementName, attr.LocalName())
			}
			continue
		}
		if !allowed[attr.LocalName()] {
			return fmt.Errorf("%s: unexpected attribute '%s'", elementName, attr.LocalName())
		}
	}
	return nil
}

var validNotationAttributes = map[string]bool{
	"id":     true,
	"name":   true,
	"public": true,
	"system": true,
}
