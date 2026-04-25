package schemaast

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/value"
)

// validateAnnotationOrder checks that annotation (if present) is the first XSD child element.
// Per XSD spec, annotation must appear first in element, attribute, complexType, simpleType, etc.
func validateAnnotationOrder(doc *Document, elem NodeID) error {
	seenNonAnnotation := false
	seenAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != value.XSDNamespace {
			continue
		}

		if doc.LocalName(child) == "annotation" {
			if seenNonAnnotation {
				return fmt.Errorf("annotation must be first child element, found after other XSD elements")
			}
			if seenAnnotation {
				return fmt.Errorf("at most one annotation element is allowed")
			}
			seenAnnotation = true
		} else {
			seenNonAnnotation = true
		}
	}
	return nil
}

func validateOnlyAnnotationChildren(doc *Document, elem NodeID, elementName string) error {
	seenAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != value.XSDNamespace {
			continue
		}
		childName := doc.LocalName(child)
		if childName == "annotation" {
			if seenAnnotation {
				return fmt.Errorf("%s: at most one annotation is allowed", elementName)
			}
			seenAnnotation = true
			continue
		}
		return fmt.Errorf("%s: unexpected child element '%s'", elementName, childName)
	}
	return nil
}
