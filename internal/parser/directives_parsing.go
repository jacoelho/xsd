package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseDirectives(doc *xsdxml.Document, root xsdxml.NodeID, schema *Schema, result *ParseResult) (map[types.NamespaceURI]bool, error) {
	importedNamespaces := make(map[types.NamespaceURI]bool)
	declIndex := 0
	includeIndex := 0
	for _, child := range doc.Children(root) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		localName := doc.LocalName(child)
		switch localName {
		case "annotation":
		case "import":
			if err := validateElementConstraints(doc, child, "import", schema); err != nil {
				return nil, err
			}
			namespace := types.ApplyWhiteSpace(doc.GetAttribute(child, "namespace"), types.WhiteSpaceCollapse)
			schemaLocation := types.ApplyWhiteSpace(doc.GetAttribute(child, "schemaLocation"), types.WhiteSpaceCollapse)
			importInfo := ImportInfo{Namespace: namespace, SchemaLocation: schemaLocation}
			result.Imports = append(result.Imports, importInfo)
			result.Directives = append(result.Directives, Directive{Kind: DirectiveImport, Import: importInfo})
			importedNamespaces[types.NamespaceURI(importInfo.Namespace)] = true
		case "include":
			if err := validateElementConstraints(doc, child, "include", schema); err != nil {
				return nil, err
			}
			schemaLocation := types.ApplyWhiteSpace(doc.GetAttribute(child, "schemaLocation"), types.WhiteSpaceCollapse)
			includeInfo := IncludeInfo{SchemaLocation: schemaLocation, DeclIndex: declIndex, IncludeIndex: includeIndex}
			if includeInfo.SchemaLocation == "" {
				return nil, fmt.Errorf("include directive missing schemaLocation")
			}
			result.Includes = append(result.Includes, includeInfo)
			result.Directives = append(result.Directives, Directive{Kind: DirectiveInclude, Include: includeInfo})
			includeIndex++
		case "element":
		case "complexType", "simpleType", "group", "attribute", "attributeGroup", "notation", "key", "keyref", "unique":
		case "redefine":
			return nil, fmt.Errorf("redefine is not supported")
		default:
			return nil, fmt.Errorf("unexpected top-level element '%s'", localName)
		}
		if isGlobalDeclElement(localName) {
			declIndex++
		}
	}
	return importedNamespaces, nil
}

func isGlobalDeclElement(localName string) bool {
	switch localName {
	case "element", "complexType", "simpleType", "group", "attribute", "attributeGroup", "notation":
		return true
	default:
		return false
	}
}

func applyImportedNamespaces(schema *Schema, importedNamespaces map[types.NamespaceURI]bool) {
	if schema.ImportedNamespaces == nil {
		schema.ImportedNamespaces = make(map[types.NamespaceURI]map[types.NamespaceURI]bool)
	}
	if schema.ImportedNamespaces[schema.TargetNamespace] == nil {
		schema.ImportedNamespaces[schema.TargetNamespace] = make(map[types.NamespaceURI]bool)
	}
	for ns := range importedNamespaces {
		schema.ImportedNamespaces[schema.TargetNamespace][ns] = true
	}
}
