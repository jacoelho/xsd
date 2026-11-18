package schema

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// validateSchemaTargetNamespace validates the raw xs:schema targetNamespace
// attribute value.
func validateSchemaTargetNamespace(target lexicalAttribute) error {
	if target.Present && target.Value == "" {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "schema targetNamespace cannot be empty")
	}
	return nil
}

// validateSchemaImportNamespace validates the raw xs:import namespace
// attribute value.
func validateSchemaImportNamespace(namespace lexicalAttribute) error {
	if namespace.Present && namespace.Value == "" {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "import namespace cannot be empty")
	}
	return nil
}

// validateSchemaNameAttribute validates the NCName syntax of an unqualified
// id or name attribute; any other attribute passes.
func validateSchemaNameAttribute(attr, value string) error {
	switch {
	case attr == vocab.XSDAttrID && !lex.IsNCName(value):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "schema id must be NCName")
	case attr == vocab.XSDAttrName && !lex.IsNCName(value):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "schema component name must be NCName")
	}
	return nil
}
