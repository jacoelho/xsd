package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func parseComplexContent(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) (*model.ComplexContent, error) {
	cc := &model.ComplexContent{}

	if err := validateOptionalID(doc, elem, "complexContent", schema); err != nil {
		return nil, err
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "mixed"); err != nil {
		return nil, err
	} else if ok {
		cc.Mixed = value
		cc.MixedSpecified = true
	}

	seenDerivation := false
	seenAnnotation := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if seenDerivation {
				return nil, fmt.Errorf("complexContent: annotation must appear before restriction or extension")
			}
			if seenAnnotation {
				return nil, fmt.Errorf("complexContent: at most one annotation is allowed")
			}
			seenAnnotation = true
		case "restriction":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("complexContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			restriction, baseQName, err := parseComplexContentRestriction(doc, child, schema)
			if err != nil {
				return nil, err
			}
			cc.Base = baseQName
			cc.Restriction = restriction
		case "extension":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("complexContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			extension, baseQName, err := parseComplexContentExtension(doc, child, schema)
			if err != nil {
				return nil, err
			}
			cc.Base = baseQName
			cc.Extension = extension
		default:
			return nil, fmt.Errorf("complexContent has unexpected child element '%s'", doc.LocalName(child))
		}
	}

	if !seenDerivation {
		return nil, fmt.Errorf("complexContent must have exactly one derivation child (restriction or extension)")
	}

	return cc, nil
}
