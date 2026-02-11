package parser

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func applyElementConstraints(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, attrs *elementAttrScan, decl *model.ElementDecl) error {
	if attrs.hasNillable {
		value, err := parseBoolValue("nillable", attrs.nillable)
		if err != nil {
			return err
		}
		decl.Nillable = value
	}

	applyElementValueConstraintFields(doc, elem, schema, attrs.hasDefault, attrs.defaultVal, attrs.hasFixed, attrs.fixedVal, decl)
	if err := applyElementBlockDerivation(schema, decl, attrs.hasBlock, attrs.block); err != nil {
		return err
	}

	return appendElementIdentityConstraints(doc, elem, schema, decl)
}
