package schema

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/xsderrors"
)

// schemaNodeNames describes one schema node enough for compile-owned name
// syntax validation.
type schemaNodeNames struct {
	ID      string
	Name    string
	HasID   bool
	HasName bool
}

// validateSchemaTargetNamespace validates the raw xs:schema targetNamespace
// attribute value.
func validateSchemaTargetNamespace(target lexicalAttribute) error {
	if target.Present && target.Value == "" {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "schema targetNamespace cannot be empty")
	}
	return nil
}

// validateSchemaNodeNames validates schema node id/name syntax.
func validateSchemaNodeNames(node schemaNodeNames) error {
	if node.HasID && !lex.IsNCName(node.ID) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "schema id must be NCName")
	}
	if node.HasName && !lex.IsNCName(node.Name) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "schema component name must be NCName")
	}
	return nil
}
