package parser

import (
	"github.com/jacoelho/xsd/internal/model"
)

func resolveElementType(doc *Document, elem NodeID, schema *Schema, attrs *elementAttrScan) (model.Type, bool, error) {
	resolved, err := resolveElementDeclType(doc, elem, schema, attrs.typ)
	if err != nil {
		return nil, false, err
	}
	return resolved.typ, resolved.hasInline, nil
}
