package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func validateImportForNamespace(schema *parser.Schema, contextNamespace, referenceNamespace model.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if referenceNamespace == model.XSDNamespace || referenceNamespace == xsdxml.XMLNamespace {
		return nil
	}
	if referenceNamespace == "" {
		if contextNamespace == "" {
			return nil
		}
		if imports, ok := schema.ImportedNamespaces[contextNamespace]; ok && imports[model.NamespaceEmpty] {
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

func validateImportForNamespaceAtLocation(schema *parser.Schema, location string, referenceNamespace model.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if referenceNamespace == model.XSDNamespace || referenceNamespace == xsdxml.XMLNamespace {
		return nil
	}
	if location == "" || schema.ImportContexts == nil {
		return validateImportForNamespace(schema, schema.TargetNamespace, referenceNamespace)
	}
	ctx, ok := schema.ImportContexts[location]
	if !ok {
		return validateImportForNamespace(schema, schema.TargetNamespace, referenceNamespace)
	}
	if referenceNamespace == "" {
		if ctx.TargetNamespace == "" {
			return nil
		}
		if ctx.Imports != nil && ctx.Imports[model.NamespaceEmpty] {
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
