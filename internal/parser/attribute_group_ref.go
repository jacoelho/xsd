package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseAttributeGroupRefQName(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	if err := validateElementConstraints(doc, elem, "attributeGroup", schema); err != nil {
		return types.QName{}, err
	}
	ref := doc.GetAttribute(elem, "ref")
	if ref == "" {
		return types.QName{}, fmt.Errorf("attributeGroup reference missing ref attribute")
	}
	refQName, err := resolveQName(doc, ref, elem, schema)
	if err != nil {
		return types.QName{}, fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
	}
	return refQName, nil
}
