package parser

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/xml"
)

// validateSchemaAttributeNamespaces enforces that schema element attributes are unqualified.
// Per validation-rules.md section 2.3.1, any non-xmlns attribute on an XSD element must be in no namespace.
func validateSchemaAttributeNamespaces(elem xml.Element) error {
	if elem.NamespaceURI() == xml.XSDNamespace {
		for _, attr := range elem.Attributes() {
			if attr.NamespaceURI() == "xmlns" {
				continue
			}
			if attr.NamespaceURI() == "" && attr.LocalName() == "xmlns" {
				continue
			}
			if attr.NamespaceURI() == xml.XSDNamespace {
				return fmt.Errorf("schema attribute '%s' on <%s> must be unprefixed", attr.LocalName(), elem.LocalName())
			}
		}
	}

	if elem.NamespaceURI() == xml.XSDNamespace && elem.LocalName() == "annotation" {
		if err := validateAnnotationStructure(elem); err != nil {
			return err
		}
	}

	for _, child := range elem.Children() {
		if err := validateSchemaAttributeNamespaces(child); err != nil {
			return err
		}
	}

	return nil
}

func validateAnnotationStructure(elem xml.Element) error {
	if err := validateAnnotationAttributes(elem); err != nil {
		return err
	}

	for _, child := range elem.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			return fmt.Errorf("annotation: unexpected child element '%s'", child.LocalName())
		}
		switch child.LocalName() {
		case "appinfo", "documentation":
			if err := validateAnnotationChildAttributes(child); err != nil {
				return err
			}
			// Allowed.
		default:
			return fmt.Errorf("annotation: unexpected child element '%s'", child.LocalName())
		}
	}

	return nil
}

func validateAnnotationAttributes(elem xml.Element) error {
	for _, attr := range elem.Attributes() {
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

func validateAnnotationChildAttributes(elem xml.Element) error {
	switch elem.LocalName() {
	case "appinfo":
		for _, attr := range elem.Attributes() {
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
		for _, attr := range elem.Attributes() {
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
