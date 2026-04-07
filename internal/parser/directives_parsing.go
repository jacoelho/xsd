package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

type directiveState struct {
	declIndex    int
	includeIndex int
}

func (s *parseSession) parseDirectiveElement(doc *Document) error {
	child := doc.DocumentElement()
	localName := doc.LocalName(child)
	switch localName {
	case "annotation":
		return nil
	case "import":
		if err := validateElementConstraints(doc, child, "import", s.schema); err != nil {
			return err
		}
		namespace := model.ApplyWhiteSpace(doc.GetAttribute(child, "namespace"), model.WhiteSpaceCollapse)
		schemaLocation := model.ApplyWhiteSpace(doc.GetAttribute(child, "schemaLocation"), model.WhiteSpaceCollapse)
		importInfo := ImportInfo{Namespace: namespace, SchemaLocation: schemaLocation}
		s.result.Imports = append(s.result.Imports, importInfo)
		s.result.Directives = append(s.result.Directives, Directive{Kind: DirectiveImport, Import: importInfo})
		s.importedNamespaces[importInfo.Namespace] = true
		return nil
	case "include":
		if err := validateElementConstraints(doc, child, "include", s.schema); err != nil {
			return err
		}
		schemaLocation := model.ApplyWhiteSpace(doc.GetAttribute(child, "schemaLocation"), model.WhiteSpaceCollapse)
		includeInfo := IncludeInfo{
			SchemaLocation: schemaLocation,
			DeclIndex:      s.dirState.declIndex,
			IncludeIndex:   s.dirState.includeIndex,
		}
		if includeInfo.SchemaLocation == "" {
			return fmt.Errorf("include directive missing schemaLocation")
		}
		s.result.Includes = append(s.result.Includes, includeInfo)
		s.result.Directives = append(s.result.Directives, Directive{Kind: DirectiveInclude, Include: includeInfo})
		s.dirState.includeIndex++
		return nil
	default:
		return fmt.Errorf("unexpected directive element '%s'", localName)
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
