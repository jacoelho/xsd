package parser

import (
	"github.com/jacoelho/xsd/internal/model"
)

func resolveTopLevelElementType(doc *Document, elem NodeID, schema *Schema) (model.Type, bool, error) {
	resolved, err := resolveElementDeclType(doc, elem, schema, doc.GetAttribute(elem, "type"))
	if err != nil {
		return nil, false, err
	}
	return resolved.typ, resolved.hasInline || resolved.hasTypeValue, nil
}
