package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseSimpleContent(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*model.SimpleContent, error) {
	sc := &model.SimpleContent{}

	if err := validateOptionalID(doc, elem, "simpleContent", schema); err != nil {
		return nil, err
	}

	seenDerivation := false
	seenAnnotation := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if seenDerivation {
				return nil, fmt.Errorf("simpleContent: annotation must appear before restriction or extension")
			}
			if seenAnnotation {
				return nil, fmt.Errorf("simpleContent: at most one annotation is allowed")
			}
			seenAnnotation = true
		case "restriction":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("simpleContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			restriction, baseQName, err := parseSimpleContentRestriction(doc, child, schema)
			if err != nil {
				return nil, err
			}
			sc.Base = baseQName
			sc.Restriction = restriction
		case "extension":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("simpleContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			extension, baseQName, err := parseSimpleContentExtension(doc, child, schema)
			if err != nil {
				return nil, err
			}
			sc.Base = baseQName
			sc.Extension = extension
		default:
			return nil, fmt.Errorf("simpleContent has unexpected child element '%s'", doc.LocalName(child))
		}
	}

	if !seenDerivation {
		return nil, fmt.Errorf("simpleContent must have exactly one derivation child (restriction or extension)")
	}

	return sc, nil
}
