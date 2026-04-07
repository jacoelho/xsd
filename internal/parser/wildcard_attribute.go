package parser

import (
	"github.com/jacoelho/xsd/internal/model"
)

// parseAnyAttribute parses an <anyAttribute> wildcard
// Content model: (annotation?)
func parseAnyAttribute(doc *Document, elem NodeID, schema *Schema) (*model.AnyAttribute, error) {
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

	if err := validateElementConstraints(doc, elem, "anyAttribute", schema); err != nil {
		return nil, err
	}

	anyAttr := &model.AnyAttribute{
		ProcessContents: processContents,
		TargetNamespace: schema.TargetNamespace,
	}
	anyAttr.Namespace = nsConstraint
	anyAttr.NamespaceList = nsList

	return anyAttr, nil
}
