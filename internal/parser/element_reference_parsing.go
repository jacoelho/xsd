package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
	"github.com/jacoelho/xsd/internal/xmltree"
)

// parseElement parses an element reference or declaration within a content model
func parseElement(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) (*model.ElementDecl, error) {
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

func parseElementReference(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema, attrs *elementAttrScan) (*model.ElementDecl, error) {
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

func validateElementReferenceAttributes(doc *xmltree.Document, elem xmltree.NodeID, attrs *elementAttrScan) error {
	if attrs.invalidRefAttr != "" {
		return fmt.Errorf("invalid attribute '%s' on element reference", attrs.invalidRefAttr)
	}
	if err := validateOnlyAnnotationChildren(doc, elem, "element"); err != nil {
		return err
	}
	return validateElementReferenceConflicts(attrs)
}

func parseElementOccurs(attrs *elementAttrScan) (occurs.Occurs, occurs.Occurs, error) {
	minOccurs := occurs.OccursFromInt(1)
	if attrs.hasMinOccurs {
		var err error
		minOccurs, err = parseOccursValue("minOccurs", attrs.minOccurs)
		if err != nil {
			return occurs.OccursFromInt(0), occurs.OccursFromInt(0), err
		}
	}
	maxOccurs := occurs.OccursFromInt(1)
	if attrs.hasMaxOccurs {
		var err error
		maxOccurs, err = parseOccursValue("maxOccurs", attrs.maxOccurs)
		if err != nil {
			return occurs.OccursFromInt(0), occurs.OccursFromInt(0), err
		}
	}
	return minOccurs, maxOccurs, nil
}
