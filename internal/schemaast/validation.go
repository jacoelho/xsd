package schemaast

import (
	"fmt"
	"github.com/jacoelho/xsd/internal/value"
)

// validateSchemaAttributeNamespaces enforces that schema element attributes are unqualified.
// Per validation-rules.md section 2.3.1, any non-xmlns attribute on an XSD element must be in no namespace.
func validateSchemaAttributeNamespaces(doc *Document, elem NodeID) error {
	if doc.NamespaceURI(elem) == value.XSDNamespace {
		for _, attr := range doc.Attributes(elem) {
			if isXMLNSDeclaration(attr) {
				continue
			}
			if attr.NamespaceURI() == value.XSDNamespace {
				return fmt.Errorf("schema attribute '%s' on <%s> must be unprefixed", attr.LocalName(), doc.LocalName(elem))
			}
		}
		if doc.LocalName(elem) == "annotation" {
			if err := validateAnnotationStructure(doc, elem); err != nil {
				return err
			}
		}
	}

	for _, child := range doc.Children(elem) {
		if err := validateSchemaAttributeNamespaces(doc, child); err != nil {
			return err
		}
	}

	return nil
}

func validateAnnotationStructure(doc *Document, elem NodeID) error {
	if err := validateAnnotationAttributes(doc, elem); err != nil {
		return err
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != value.XSDNamespace {
			return fmt.Errorf("annotation: unexpected child element '%s'", doc.LocalName(child))
		}
		localName := doc.LocalName(child)
		if localName != "appinfo" && localName != "documentation" {
			return fmt.Errorf("annotation: unexpected child element '%s'", localName)
		}
		if err := validateAnnotationChildAttributes(doc, child); err != nil {
			return err
		}
	}

	return nil
}

func validateAnnotationAttributes(doc *Document, elem NodeID) error {
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		namespace := attr.NamespaceURI()
		localName := attr.LocalName()
		if namespace == value.XSDNamespace {
			return fmt.Errorf("annotation: attribute '%s' must be unprefixed", localName)
		}
		if namespace != "" {
			continue
		}
		if localName != "id" {
			return fmt.Errorf("annotation: unexpected attribute '%s'", localName)
		}
		if TrimXMLWhitespace(attr.Value()) == "" {
			return fmt.Errorf("annotation: id attribute cannot be empty")
		}
	}
	return nil
}

func validateAnnotationChildAttributes(doc *Document, elem NodeID) error {
	elemName := doc.LocalName(elem)

	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		namespace := attr.NamespaceURI()
		localName := attr.LocalName()
		if namespace == value.XSDNamespace {
			return fmt.Errorf("%s: attribute '%s' must be unprefixed", elemName, localName)
		}
		if namespace == "" {
			if localName != "source" {
				return fmt.Errorf("%s: unexpected attribute '%s'", elemName, localName)
			}
			continue
		}
		if elemName == "documentation" && namespace == value.XMLNamespace && localName == "lang" && TrimXMLWhitespace(attr.Value()) == "" {
			return fmt.Errorf("documentation: xml:lang must not be empty")
		}
	}
	return nil
}
