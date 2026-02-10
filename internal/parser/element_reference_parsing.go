package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

// parseElement parses an element reference or declaration within a content model
func parseElement(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) (*model.ElementDecl, error) {
	attrs := scanElementAttributes(doc, elem)

	if attrs.hasID {
		if err := validateIDAttribute(attrs.id, "element", schema); err != nil {
			return nil, err
		}
	}

	if attrs.ref != "" && attrs.name != "" {
		return nil, fmt.Errorf("element cannot have both 'name' and 'ref' attributes")
	}

	if attrs.ref != "" {
		return parseElementReference(doc, elem, schema, &attrs)
	}

	return parseLocalElement(doc, elem, schema, &attrs)
}

func parseElementReference(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, attrs *elementAttrScan) (*model.ElementDecl, error) {
	if err := validateElementReferenceAttributes(doc, elem, attrs); err != nil {
		return nil, err
	}

	refQName, err := resolveQNameWithPolicy(doc, attrs.ref, elem, schema, useDefaultNamespace)
	if err != nil {
		return nil, fmt.Errorf("resolve ref %s: %w", attrs.ref, err)
	}

	minOccurs, maxOccurs, err := parseElementOccurs(attrs)
	if err != nil {
		return nil, err
	}

	decl := &model.ElementDecl{
		Name:        refQName,
		MinOccurs:   minOccurs,
		MaxOccurs:   maxOccurs,
		IsReference: true,
	}
	parsed, err := model.NewElementDeclFromParsed(decl)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func validateElementReferenceAttributes(doc *schemaxml.Document, elem schemaxml.NodeID, attrs *elementAttrScan) error {
	if attrs.invalidRefAttr != "" {
		return fmt.Errorf("invalid attribute '%s' on element reference", attrs.invalidRefAttr)
	}
	if err := validateOnlyAnnotationChildren(doc, elem, "element"); err != nil {
		return err
	}
	if attrs.hasType {
		return fmt.Errorf("element reference cannot have 'type' attribute")
	}
	if attrs.hasDefault {
		return fmt.Errorf("element reference cannot have 'default' attribute")
	}
	if attrs.hasFixed {
		return fmt.Errorf("element reference cannot have 'fixed' attribute")
	}
	if attrs.hasNillable {
		return fmt.Errorf("element reference cannot have 'nillable' attribute")
	}
	if attrs.hasBlock {
		return fmt.Errorf("element reference cannot have 'block' attribute")
	}
	if attrs.hasFinal {
		return fmt.Errorf("element reference cannot have 'final' attribute")
	}
	if attrs.hasForm {
		return fmt.Errorf("element reference cannot have 'form' attribute")
	}
	if attrs.hasAbstract {
		return fmt.Errorf("element reference cannot have 'abstract' attribute")
	}
	return nil
}

func parseElementOccurs(attrs *elementAttrScan) (model.Occurs, model.Occurs, error) {
	minOccurs := model.OccursFromInt(1)
	if attrs.hasMinOccurs {
		var err error
		minOccurs, err = parseOccursValue("minOccurs", attrs.minOccurs)
		if err != nil {
			return model.OccursFromInt(0), model.OccursFromInt(0), err
		}
	}
	maxOccurs := model.OccursFromInt(1)
	if attrs.hasMaxOccurs {
		var err error
		maxOccurs, err = parseOccursValue("maxOccurs", attrs.maxOccurs)
		if err != nil {
			return model.OccursFromInt(0), model.OccursFromInt(0), err
		}
	}
	return minOccurs, maxOccurs, nil
}
