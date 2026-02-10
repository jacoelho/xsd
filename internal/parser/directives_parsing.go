package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func parseDirectives(doc *schemaxml.Document, root schemaxml.NodeID, schema *Schema, result *ParseResult) (map[model.NamespaceURI]bool, error) {
	importedNamespaces := make(map[model.NamespaceURI]bool)
	state := directiveState{}
	for _, child := range doc.Children(root) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}
		if err := parseDirectiveElement(doc, child, schema, result, importedNamespaces, &state); err != nil {
			return nil, err
		}
	}
	return importedNamespaces, nil
}

type directiveState struct {
	declIndex    int
	includeIndex int
}

func parseDirectiveElement(doc *schemaxml.Document, child schemaxml.NodeID, schema *Schema, result *ParseResult, importedNamespaces map[model.NamespaceURI]bool, state *directiveState) error {
	if doc == nil {
		return fmt.Errorf("directive document missing")
	}
	if state == nil {
		return fmt.Errorf("directive state missing")
	}
	localName := doc.LocalName(child)
	switch localName {
	case "annotation":
	case "import":
		if err := validateElementConstraints(doc, child, "import", schema); err != nil {
			return err
		}
		namespace := model.ApplyWhiteSpace(doc.GetAttribute(child, "namespace"), model.WhiteSpaceCollapse)
		schemaLocation := model.ApplyWhiteSpace(doc.GetAttribute(child, "schemaLocation"), model.WhiteSpaceCollapse)
		importInfo := ImportInfo{Namespace: namespace, SchemaLocation: schemaLocation}
		result.Imports = append(result.Imports, importInfo)
		result.Directives = append(result.Directives, Directive{Kind: DirectiveImport, Import: importInfo})
		importedNamespaces[importInfo.Namespace] = true
	case "include":
		if err := validateElementConstraints(doc, child, "include", schema); err != nil {
			return err
		}
		schemaLocation := model.ApplyWhiteSpace(doc.GetAttribute(child, "schemaLocation"), model.WhiteSpaceCollapse)
		includeInfo := IncludeInfo{
			SchemaLocation: schemaLocation,
			DeclIndex:      state.declIndex,
			IncludeIndex:   state.includeIndex,
		}
		if includeInfo.SchemaLocation == "" {
			return fmt.Errorf("include directive missing schemaLocation")
		}
		result.Includes = append(result.Includes, includeInfo)
		result.Directives = append(result.Directives, Directive{Kind: DirectiveInclude, Include: includeInfo})
		state.includeIndex++
	case "element":
	case "complexType", "simpleType", "group", "attribute", "attributeGroup", "notation", "key", "keyref", "unique":
	case "redefine":
		return fmt.Errorf("redefine is not supported")
	default:
		return fmt.Errorf("unexpected top-level element '%s'", localName)
	}
	if isGlobalDeclElement(localName) {
		state.declIndex++
	}
	return nil
}

func isGlobalDeclElement(localName string) bool {
	switch localName {
	case "element", "complexType", "simpleType", "group", "attribute", "attributeGroup", "notation":
		return true
	default:
		return false
	}
}

func applyImportedNamespaces(schema *Schema, importedNamespaces map[model.NamespaceURI]bool) {
	if schema.ImportedNamespaces == nil {
		schema.ImportedNamespaces = make(map[model.NamespaceURI]map[model.NamespaceURI]bool)
	}
	if schema.ImportedNamespaces[schema.TargetNamespace] == nil {
		schema.ImportedNamespaces[schema.TargetNamespace] = make(map[model.NamespaceURI]bool)
	}
	for ns := range importedNamespaces {
		schema.ImportedNamespaces[schema.TargetNamespace][ns] = true
	}
}
