package parser

import (
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func resolveElementType(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema, attrs *elementAttrScan) (model.Type, bool, error) {
	resolved, err := resolveElementDeclType(doc, elem, schema, attrs.typ)
	if err != nil {
		return nil, false, err
	}
	return resolved.typ, resolved.hasInline, nil
}
