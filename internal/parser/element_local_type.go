package parser

import (
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func resolveElementType(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, attrs *elementAttrScan) (model.Type, bool, error) {
	resolved, err := resolveElementDeclType(doc, elem, schema, attrs.typ)
	if err != nil {
		return nil, false, err
	}
	return resolved.typ, resolved.hasInline, nil
}
