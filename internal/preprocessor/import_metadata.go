package preprocessor

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func ensureNamespaceMap(m map[model.NamespaceURI]map[model.NamespaceURI]bool, key model.NamespaceURI) map[model.NamespaceURI]bool {
	if m[key] == nil {
		m[key] = make(map[model.NamespaceURI]bool)
	}
	return m[key]
}

func registerImports(sch *parser.Schema, imports []parser.ImportInfo) {
	if sch == nil {
		return
	}
	if sch.ImportedNamespaces == nil {
		sch.ImportedNamespaces = make(map[model.NamespaceURI]map[model.NamespaceURI]bool)
	}
	fromNS := sch.TargetNamespace
	imported := ensureNamespaceMap(sch.ImportedNamespaces, fromNS)
	for _, imp := range imports {
		ns := imp.Namespace
		imported[ns] = true
	}

	if sch.ImportContexts == nil {
		sch.ImportContexts = make(map[string]parser.ImportContext)
	}
	if sch.Location != "" {
		ctx := sch.ImportContexts[sch.Location]
		if ctx.Imports == nil {
			ctx.Imports = make(map[model.NamespaceURI]bool)
		}
		ctx.TargetNamespace = sch.TargetNamespace
		for _, imp := range imports {
			ns := imp.Namespace
			ctx.Imports[ns] = true
		}
		sch.ImportContexts[sch.Location] = ctx
	}
}

func validateImportConstraints(sch *parser.Schema, imports []parser.ImportInfo) error {
	if sch.TargetNamespace == "" {
		for _, imp := range imports {
			if imp.Namespace == "" {
				return fmt.Errorf("schema without targetNamespace cannot use import without namespace attribute (namespace attribute is required)")
			}
		}
	}
	for _, imp := range imports {
		if imp.Namespace == "" {
			continue
		}
		if sch.TargetNamespace != "" && imp.Namespace == sch.TargetNamespace {
			return fmt.Errorf("import namespace %s must be different from target namespace", imp.Namespace)
		}
	}
	return nil
}
