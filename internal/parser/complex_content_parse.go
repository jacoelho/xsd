package parser

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func parseComplexContent(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) (*model.ComplexContent, error) {
	cc := &model.ComplexContent{}

	if err := validateOptionalID(doc, elem, "complexContent", schema); err != nil {
		return nil, err
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "mixed"); err != nil {
		return nil, err
	} else if ok {
		cc.Mixed = value
		cc.MixedSpecified = true
	}

	parsed, err := parseDerivationContent(doc, elem, schema, "complexContent", parseComplexContentRestriction, parseComplexContentExtension)
	if err != nil {
		return nil, err
	}

	cc.Base = parsed.base
	cc.Restriction = parsed.restriction
	cc.Extension = parsed.extension

	return cc, nil
}
