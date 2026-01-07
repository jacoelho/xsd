package parser

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/xml"
)

// validateSchemaAttributeNamespaces enforces that schema element attributes are unqualified.
// Per validation-rules.md section 2.3.1, any non-xmlns attribute on an XSD element must be in no namespace.
func validateSchemaAttributeNamespaces(doc *xml.Document, elem xml.NodeID) error {
	if doc.NamespaceURI(elem) == xml.XSDNamespace {
		for _, attr := range doc.Attributes(elem) {
			if attr.NamespaceURI() == "xmlns" {
				continue
			}
			if attr.NamespaceURI() == "" && attr.LocalName() == "xmlns" {
				continue
			}
			if attr.NamespaceURI() == xml.XSDNamespace {
				return fmt.Errorf("schema attribute '%s' on <%s> must be unprefixed", attr.LocalName(), doc.LocalName(elem))
			}
		}
	}

	if doc.NamespaceURI(elem) == xml.XSDNamespace && doc.LocalName(elem) == "annotation" {
		if err := validateAnnotationStructure(doc, elem); err != nil {
			return err
		}
	}

	for _, child := range doc.Children(elem) {
		if err := validateSchemaAttributeNamespaces(doc, child); err != nil {
			return err
		}
	}

	return nil
}

func validateAnnotationStructure(doc *xml.Document, elem xml.NodeID) error {
	if err := validateAnnotationAttributes(doc, elem); err != nil {
		return err
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			return fmt.Errorf("annotation: unexpected child element '%s'", doc.LocalName(child))
		}
		switch doc.LocalName(child) {
		case "appinfo", "documentation":
			if err := validateAnnotationChildAttributes(doc, child); err != nil {
				return err
			}
			// allowed.
		default:
			return fmt.Errorf("annotation: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	return nil
}

func validateAnnotationAttributes(doc *xml.Document, elem xml.NodeID) error {
	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() == "xmlns" {
			continue
		}
		if attr.NamespaceURI() == "" && attr.LocalName() == "xmlns" {
			continue
		}
		if attr.NamespaceURI() == "" {
			if attr.LocalName() != "id" {
				return fmt.Errorf("annotation: unexpected attribute '%s'", attr.LocalName())
			}
			if strings.TrimSpace(attr.Value()) == "" {
				return fmt.Errorf("annotation: id attribute cannot be empty")
			}
			continue
		}
		if attr.NamespaceURI() == xml.XSDNamespace {
			return fmt.Errorf("annotation: attribute '%s' must be unprefixed", attr.LocalName())
		}
	}
	return nil
}

func validateAnnotationChildAttributes(doc *xml.Document, elem xml.NodeID) error {
	switch doc.LocalName(elem) {
	case "appinfo":
		for _, attr := range doc.Attributes(elem) {
			if attr.NamespaceURI() == "xmlns" {
				continue
			}
			if attr.NamespaceURI() == "" && attr.LocalName() == "xmlns" {
				continue
			}
			if attr.NamespaceURI() == "" && attr.LocalName() != "source" {
				return fmt.Errorf("appinfo: unexpected attribute '%s'", attr.LocalName())
			}
			if attr.NamespaceURI() == xml.XSDNamespace {
				return fmt.Errorf("appinfo: attribute '%s' must be unprefixed", attr.LocalName())
			}
		}
	case "documentation":
		for _, attr := range doc.Attributes(elem) {
			if attr.NamespaceURI() == "xmlns" {
				continue
			}
			if attr.NamespaceURI() == "" && attr.LocalName() == "xmlns" {
				continue
			}
			if attr.NamespaceURI() == "" {
				if attr.LocalName() != "source" {
					return fmt.Errorf("documentation: unexpected attribute '%s'", attr.LocalName())
				}
				continue
			}
			if attr.NamespaceURI() == xml.XMLNamespace && attr.LocalName() == "lang" {
				if strings.TrimSpace(attr.Value()) == "" {
					return fmt.Errorf("documentation: xml:lang must not be empty")
				}
				continue
			}
			if attr.NamespaceURI() == xml.XSDNamespace {
				return fmt.Errorf("documentation: attribute '%s' must be unprefixed", attr.LocalName())
			}
		}
	}
	return nil
}
