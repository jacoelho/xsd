package parser

import (
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func resolveTopLevelElementType(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) (model.Type, bool, error) {
	resolved, err := resolveElementDeclType(doc, elem, schema, doc.GetAttribute(elem, "type"))
	if err != nil {
		return nil, false, err
	}
	return resolved.typ, resolved.hasInline || resolved.hasTypeValue, nil
}
