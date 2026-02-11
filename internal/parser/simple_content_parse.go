package parser

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func parseSimpleContent(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) (*model.SimpleContent, error) {
	sc := &model.SimpleContent{}

	if err := validateOptionalID(doc, elem, "simpleContent", schema); err != nil {
		return nil, err
	}

	parsed, err := parseDerivationContent(doc, elem, schema, "simpleContent", parseSimpleContentRestriction, parseSimpleContentExtension)
	if err != nil {
		return nil, err
	}

	sc.Base = parsed.base
	sc.Restriction = parsed.restriction
	sc.Extension = parsed.extension

	return sc, nil
}
