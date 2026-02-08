package source

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func ensureNamespaceMap(m map[types.NamespaceURI]map[types.NamespaceURI]bool, key types.NamespaceURI) map[types.NamespaceURI]bool {
	if m[key] == nil {
		m[key] = make(map[types.NamespaceURI]bool)
	}
	return m[key]
}

func registerImports(sch *parser.Schema, imports []parser.ImportInfo) {
	if sch == nil {
		return
	}
	if sch.ImportedNamespaces == nil {
		sch.ImportedNamespaces = make(map[types.NamespaceURI]map[types.NamespaceURI]bool)
	}
	fromNS := sch.TargetNamespace
	imported := ensureNamespaceMap(sch.ImportedNamespaces, fromNS)
	for _, imp := range imports {
		ns := types.NamespaceURI(imp.Namespace)
		imported[ns] = true
	}

	if sch.ImportContexts == nil {
		sch.ImportContexts = make(map[string]parser.ImportContext)
	}
	if sch.Location != "" {
		ctx := sch.ImportContexts[sch.Location]
		if ctx.Imports == nil {
			ctx.Imports = make(map[types.NamespaceURI]bool)
		}
		ctx.TargetNamespace = sch.TargetNamespace
		for _, imp := range imports {
			ns := types.NamespaceURI(imp.Namespace)
			ctx.Imports[ns] = true
		}
		sch.ImportContexts[sch.Location] = ctx
	}
}

func validateImportConstraints(sch *parser.Schema, imports []parser.ImportInfo) error {
	if sch.TargetNamespace.IsEmpty() {
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
		if !sch.TargetNamespace.IsEmpty() && types.NamespaceURI(imp.Namespace) == sch.TargetNamespace {
			return fmt.Errorf("import namespace %s must be different from target namespace", imp.Namespace)
		}
	}
	return nil
}

func initSchemaOrigins(sch *parser.Schema, location string) {
	if sch == nil {
		return
	}
	sch.Location = parser.ImportContextKey("", location)
	for _, qname := range sortedQNames(sch.ElementDecls) {
		if sch.ElementOrigins[qname] == "" {
			sch.ElementOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.TypeDefs) {
		if sch.TypeOrigins[qname] == "" {
			sch.TypeOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.AttributeDecls) {
		if sch.AttributeOrigins[qname] == "" {
			sch.AttributeOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.AttributeGroups) {
		if sch.AttributeGroupOrigins[qname] == "" {
			sch.AttributeGroupOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.Groups) {
		if sch.GroupOrigins[qname] == "" {
			sch.GroupOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.NotationDecls) {
		if sch.NotationOrigins[qname] == "" {
			sch.NotationOrigins[qname] = sch.Location
		}
	}
}

func sortedQNames[V any](m map[types.QName]V) []types.QName {
	keys := make([]types.QName, 0, len(m))
	for qname := range m {
		keys = append(keys, qname)
	}
	slices.SortFunc(keys, types.CompareQName)
	return keys
}
