package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

// parseAnyAttribute parses an <anyAttribute> wildcard
// Content model: (annotation?)
func parseAnyAttribute(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.AnyAttribute, error) {
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
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
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

	anyAttr := &types.AnyAttribute{
		ProcessContents: processContents,
		TargetNamespace: schema.TargetNamespace,
	}
	anyAttr.Namespace = nsConstraint
	anyAttr.NamespaceList = nsList

	return anyAttr, nil
}
