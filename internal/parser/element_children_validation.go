package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schemaxml"
)

// validateAnnotationOrder checks that annotation (if present) is the first XSD child element.
// Per XSD spec, annotation must appear first in element, attribute, complexType, simpleType, etc.
func validateAnnotationOrder(doc *schemaxml.Document, elem schemaxml.NodeID) error {
	seenNonAnnotation := false
	annotationCount := 0
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}

		if doc.LocalName(child) == "annotation" {
			if seenNonAnnotation {
				return fmt.Errorf("annotation must be first child element, found after other XSD elements")
			}
			annotationCount++
			if annotationCount > 1 {
				return fmt.Errorf("at most one annotation element is allowed")
			}
		} else {
			seenNonAnnotation = true
		}
	}
	return nil
}

// validateElementChildrenOrder checks that identity constraints follow any inline type definition.
// Per XSD 1.0, element content model is: (annotation?, (simpleType|complexType)?, (unique|key|keyref)*).
func validateElementChildrenOrder(doc *schemaxml.Document, elem schemaxml.NodeID) error {
	seenType := false
	seenConstraint := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			continue
		case "simpleType", "complexType":
			if seenConstraint {
				return fmt.Errorf("element type definition must precede identity constraints")
			}
			if seenType {
				return fmt.Errorf("element cannot have more than one inline type definition")
			}
			seenType = true
		case "unique", "key", "keyref":
			seenConstraint = true
		}
	}
	return nil
}

func validateOnlyAnnotationChildren(doc *schemaxml.Document, elem schemaxml.NodeID, elementName string) error {
	seenAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}
		if doc.LocalName(child) == "annotation" {
			if seenAnnotation {
				return fmt.Errorf("%s: at most one annotation is allowed", elementName)
			}
			seenAnnotation = true
			continue
		}
		return fmt.Errorf("%s: unexpected child element '%s'", elementName, doc.LocalName(child))
	}
	return nil
}

func validateElementConstraints(doc *schemaxml.Document, elem schemaxml.NodeID, elementName string, schema *Schema) error {
	if err := validateOptionalID(doc, elem, elementName, schema); err != nil {
		return err
	}
	if err := validateOnlyAnnotationChildren(doc, elem, elementName); err != nil {
		return err
	}
	return nil
}
