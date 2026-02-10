package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func parseAttributeGroupRefQName(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) (model.QName, error) {
	if err := validateElementConstraints(doc, elem, "attributeGroup", schema); err != nil {
		return model.QName{}, err
	}
	ref := doc.GetAttribute(elem, "ref")
	if ref == "" {
		return model.QName{}, fmt.Errorf("attributeGroup reference missing ref attribute")
	}
	refQName, err := resolveQNameWithPolicy(doc, ref, elem, schema, useDefaultNamespace)
	if err != nil {
		return model.QName{}, fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
	}
	return refQName, nil
}
