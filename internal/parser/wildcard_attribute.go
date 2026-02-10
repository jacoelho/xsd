package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

// parseAnyAttribute parses an <anyAttribute> wildcard
// Content model: (annotation?)
func parseAnyAttribute(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) (*model.AnyAttribute, error) {
	nsConstraint, nsList, processContents, err := parseWildcardConstraints(
		doc,
		elem,
		"anyAttribute",
		"namespace, processContents",
		validAttributeNames[attrSetAnyAttribute],
	)
	if err != nil {
		return nil, err
	}

	if err := validateOptionalID(doc, elem, "anyAttribute", schema); err != nil {
		return nil, err
	}

	hasAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return nil, fmt.Errorf("anyAttribute: at most one annotation is allowed")
			}
			hasAnnotation = true
		default:
			return nil, fmt.Errorf("anyAttribute: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	anyAttr := &model.AnyAttribute{
		ProcessContents: processContents,
		TargetNamespace: schema.TargetNamespace,
	}
	anyAttr.Namespace = nsConstraint
	anyAttr.NamespaceList = nsList

	return anyAttr, nil
}
