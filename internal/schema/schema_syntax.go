package schema

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// SchemaNodeNames describes one schema node enough for compile-owned name
// syntax validation.
type SchemaNodeNames struct {
	Local       string
	ParentLocal string
	ID          string
	Name        string
	XSD         bool
	ParentXSD   bool
	HasID       bool
	HasName     bool
}

// ValidateSchemaTargetNamespace validates the raw xs:schema targetNamespace
// attribute value.
func ValidateSchemaTargetNamespace(target LexicalAttribute) error {
	if target.Present && target.Value == "" {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "schema targetNamespace cannot be empty")
	}
	return nil
}

// ValidateSchemaNodeNames validates schema node id/name syntax and name
// placement rules.
func ValidateSchemaNodeNames(node SchemaNodeNames) error {
	if !node.XSD {
		return nil
	}
	if node.HasID && !lex.IsNCName(node.ID) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "schema id must be NCName")
	}
	if node.HasName && !lex.IsNCName(node.Name) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "schema component name must be NCName")
	}
	if node.Local == attributeGroup && node.HasName && (!node.ParentXSD || node.ParentLocal != vocab.XSDElemSchema) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "attributeGroup use cannot have name")
	}
	return nil
}
