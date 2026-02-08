package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func validateImportForNamespace(schema *parser.Schema, contextNamespace, referenceNamespace types.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if referenceNamespace == types.XSDNamespace || referenceNamespace == xsdxml.XMLNamespace {
		return nil
	}
	if referenceNamespace.IsEmpty() {
		if contextNamespace.IsEmpty() {
			return nil
		}
		if imports, ok := schema.ImportedNamespaces[contextNamespace]; ok && imports[types.NamespaceEmpty] {
			return nil
		}
		return fmt.Errorf("namespace %s not imported for %s", referenceNamespace, contextNamespace)
	}
	if referenceNamespace == contextNamespace {
		return nil
	}
	if imports, ok := schema.ImportedNamespaces[contextNamespace]; ok && imports[referenceNamespace] {
		return nil
	}
	return fmt.Errorf("namespace %s not imported for %s", referenceNamespace, contextNamespace)
}

func validateImportForNamespaceAtLocation(schema *parser.Schema, location string, referenceNamespace types.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if referenceNamespace == types.XSDNamespace || referenceNamespace == xsdxml.XMLNamespace {
		return nil
	}
	if location == "" || schema.ImportContexts == nil {
		return validateImportForNamespace(schema, schema.TargetNamespace, referenceNamespace)
	}
	ctx, ok := schema.ImportContexts[location]
	if !ok {
		return validateImportForNamespace(schema, schema.TargetNamespace, referenceNamespace)
	}
	if referenceNamespace.IsEmpty() {
		if ctx.TargetNamespace.IsEmpty() {
			return nil
		}
		if ctx.Imports != nil && ctx.Imports[types.NamespaceEmpty] {
			return nil
		}
		return fmt.Errorf("namespace %s must be imported by schema %s", referenceNamespace, parser.ImportContextLocation(location))
	}
	if referenceNamespace == ctx.TargetNamespace {
		return nil
	}
	if ctx.Imports != nil && ctx.Imports[referenceNamespace] {
		return nil
	}
	return fmt.Errorf("namespace %s must be imported by schema %s", referenceNamespace, parser.ImportContextLocation(location))
}
