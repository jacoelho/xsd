package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/value"
)

const noOriginLocation = ""

func validateImportForNamespace(schema *parser.Schema, contextNamespace, referenceNamespace model.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if referenceNamespace == model.XSDNamespace || referenceNamespace == value.XMLNamespace {
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
	if referenceNamespace == model.XSDNamespace || referenceNamespace == value.XMLNamespace {
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

// validateTypeQNameReference validates that a type QName reference exists.
func validateTypeQNameReference(schema *parser.Schema, qname model.QName, contextNamespace model.NamespaceURI) error {
	if qname.IsZero() {
		return nil
	}
	if err := validateImportForNamespace(schema, contextNamespace, qname.Namespace); err != nil {
		return err
	}
	return parser.ValidateTypeQName(schema, qname)
}

func validateTypeReferenceFromTypeAtLocation(schema *parser.Schema, typ model.Type, contextNamespace model.NamespaceURI, originLocation string) error {
	visited := make(map[*model.ModelGroup]bool)
	return validateTypeReferenceFromTypeWithVisited(schema, typ, visited, contextNamespace, originLocation)
}

// validateTypeReferenceFromTypeWithVisited validates type reference with cycle detection.
func validateTypeReferenceFromTypeWithVisited(schema *parser.Schema, typ model.Type, visited map[*model.ModelGroup]bool, contextNamespace model.NamespaceURI, originLocation string) error {
	if typ == nil {
		return nil
	}
	if err := validateTypeImportReference(schema, typ, contextNamespace); err != nil {
		return err
	}
	if typ.IsBuiltin() {
		return nil
	}
	if err := validateSimpleTypeQNameReference(schema, typ); err != nil {
		return err
	}
	return validateComplexTypeContentReferences(schema, typ, visited, originLocation)
}

func validateTypeImportReference(schema *parser.Schema, typ model.Type, contextNamespace model.NamespaceURI) error {
	qname := typ.Name()
	if qname.IsZero() {
		return nil
	}
	return validateImportForNamespace(schema, contextNamespace, qname.Namespace)
}

func validateSimpleTypeQNameReference(schema *parser.Schema, typ model.Type) error {
	if st, ok := model.AsSimpleType(typ); ok {
		if !st.IsBuiltin() && st.Restriction == nil && st.List == nil && st.Union == nil {
			if err := parser.ValidateTypeQName(schema, st.QName); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateComplexTypeContentReferences(schema *parser.Schema, typ model.Type, visited map[*model.ModelGroup]bool, originLocation string) error {
	if ct, ok := model.AsComplexType(typ); ok {
		if content := ct.Content(); content != nil {
			return validateComplexTypeElementContentReferences(schema, content, visited, originLocation)
		}
	}
	return nil
}

func validateComplexTypeElementContentReferences(schema *parser.Schema, content model.Content, visited map[*model.ModelGroup]bool, originLocation string) error {
	ec, ok := content.(*model.ElementContent)
	if !ok || ec.Particle == nil {
		return nil
	}
	return validateParticleReferencesWithVisited(schema, ec.Particle, visited, originLocation)
}
