package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func validateLocalElementAttributes(attrs *elementAttrScan) error {
	if attrs.hasAbstract {
		return fmt.Errorf("local element cannot have 'abstract' attribute (only global elements can be abstract)")
	}
	if attrs.hasFinal {
		return fmt.Errorf("local element cannot have 'final' attribute (only global elements can be final)")
	}
	if attrs.hasDefault && attrs.hasFixed {
		return fmt.Errorf("element cannot have both 'default' and 'fixed' attributes")
	}
	return nil
}

func validateLocalElementChildren(doc *schemaxml.Document, elem schemaxml.NodeID) error {
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return err
	}
	if err := validateElementChildrenOrder(doc, elem); err != nil {
		return err
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation", "complexType", "simpleType", "key", "keyref", "unique":
			continue
		default:
			return fmt.Errorf("invalid child element <%s> in <element> declaration", doc.LocalName(child))
		}
	}

	return nil
}

func resolveLocalElementForm(attrs *elementAttrScan, schema *Schema) (Form, model.NamespaceURI, error) {
	var effectiveForm Form
	if formAttr := attrs.form; formAttr != "" {
		formAttr = model.ApplyWhiteSpace(formAttr, model.WhiteSpaceCollapse)
		switch formAttr {
		case "qualified":
			effectiveForm = Qualified
		case "unqualified":
			effectiveForm = Unqualified
		default:
			return Unqualified, "", fmt.Errorf("invalid form attribute value '%s': must be 'qualified' or 'unqualified'", formAttr)
		}
	} else {
		effectiveForm = schema.ElementFormDefault
	}

	var elementNamespace model.NamespaceURI
	if effectiveForm == Qualified {
		elementNamespace = schema.TargetNamespace
	}

	return effectiveForm, elementNamespace, nil
}
