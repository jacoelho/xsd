package parser

import (
	"maps"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/qname"
)

// CloneSchemaForMerge returns a schema copy suitable for in-place merge staging.
func CloneSchemaForMerge(sch *Schema) *Schema {
	if sch == nil {
		return nil
	}
	clone := *sch
	clone.ImportContexts = copyImportContexts(sch.ImportContexts)
	clone.ImportedNamespaces = copyImportedNamespaces(sch.ImportedNamespaces)
	clone.ElementDecls = maps.Clone(sch.ElementDecls)
	clone.ElementOrigins = maps.Clone(sch.ElementOrigins)
	clone.TypeDefs = maps.Clone(sch.TypeDefs)
	clone.TypeOrigins = maps.Clone(sch.TypeOrigins)
	clone.AttributeDecls = maps.Clone(sch.AttributeDecls)
	clone.AttributeOrigins = maps.Clone(sch.AttributeOrigins)
	clone.AttributeGroups = maps.Clone(sch.AttributeGroups)
	clone.AttributeGroupOrigins = maps.Clone(sch.AttributeGroupOrigins)
	clone.Groups = maps.Clone(sch.Groups)
	clone.GroupOrigins = maps.Clone(sch.GroupOrigins)
	clone.SubstitutionGroups = copyQNameSliceMap(sch.SubstitutionGroups)
	clone.NotationDecls = maps.Clone(sch.NotationDecls)
	clone.NotationOrigins = maps.Clone(sch.NotationOrigins)
	clone.IDAttributes = maps.Clone(sch.IDAttributes)
	clone.GlobalDecls = slices.Clone(sch.GlobalDecls)
	return &clone
}

// CloneSchema deep-clones a parsed schema and its mutable declaration graph.
func CloneSchema(sch *Schema) *Schema {
	if sch == nil {
		return nil
	}

	clone := NewSchema()
	clone.Location = sch.Location
	clone.TargetNamespace = sch.TargetNamespace
	clone.AttributeFormDefault = sch.AttributeFormDefault
	clone.ElementFormDefault = sch.ElementFormDefault
	clone.BlockDefault = sch.BlockDefault
	clone.FinalDefault = sch.FinalDefault
	clone.NamespaceDecls = maps.Clone(sch.NamespaceDecls)
	clone.ImportContexts = copyImportContexts(sch.ImportContexts)
	clone.ImportedNamespaces = copyImportedNamespaces(sch.ImportedNamespaces)
	clone.IDAttributes = maps.Clone(sch.IDAttributes)
	clone.GlobalDecls = slices.Clone(sch.GlobalDecls)
	clone.SubstitutionGroups = copyQNameSliceMap(sch.SubstitutionGroups)

	opts := model.WithGraphMemo(model.CopyOptions{
		RemapQName:              model.NilRemap,
		SourceNamespace:         sch.TargetNamespace,
		PreserveSourceNamespace: true,
	})

	for _, name := range qname.SortedMapKeys(sch.ElementDecls) {
		clone.ElementDecls[name] = sch.ElementDecls[name].Copy(opts)
	}
	maps.Copy(clone.ElementOrigins, sch.ElementOrigins)

	for _, name := range qname.SortedMapKeys(sch.TypeDefs) {
		clone.TypeDefs[name] = model.CopyType(sch.TypeDefs[name], opts)
	}
	maps.Copy(clone.TypeOrigins, sch.TypeOrigins)

	for _, name := range qname.SortedMapKeys(sch.AttributeDecls) {
		clone.AttributeDecls[name] = sch.AttributeDecls[name].Copy(opts)
	}
	maps.Copy(clone.AttributeOrigins, sch.AttributeOrigins)

	for _, name := range qname.SortedMapKeys(sch.AttributeGroups) {
		clone.AttributeGroups[name] = sch.AttributeGroups[name].Copy(opts)
	}
	maps.Copy(clone.AttributeGroupOrigins, sch.AttributeGroupOrigins)

	for _, name := range qname.SortedMapKeys(sch.Groups) {
		clone.Groups[name] = sch.Groups[name].Copy(opts)
	}
	maps.Copy(clone.GroupOrigins, sch.GroupOrigins)

	for _, name := range qname.SortedMapKeys(sch.NotationDecls) {
		clone.NotationDecls[name] = sch.NotationDecls[name].Copy(opts)
	}
	maps.Copy(clone.NotationOrigins, sch.NotationOrigins)

	return clone
}

func copyImportContexts(src map[string]ImportContext) map[string]ImportContext {
	if src == nil {
		return nil
	}
	dst := make(map[string]ImportContext, len(src))
	for key, ctx := range src {
		copied := ctx
		copied.Imports = maps.Clone(ctx.Imports)
		dst[key] = copied
	}
	return dst
}

func copyImportedNamespaces(src map[model.NamespaceURI]map[model.NamespaceURI]bool) map[model.NamespaceURI]map[model.NamespaceURI]bool {
	if src == nil {
		return nil
	}
	dst := make(map[model.NamespaceURI]map[model.NamespaceURI]bool, len(src))
	for ns, imports := range src {
		dst[ns] = maps.Clone(imports)
	}
	return dst
}

func copyQNameSliceMap(src map[model.QName][]model.QName) map[model.QName][]model.QName {
	if src == nil {
		return nil
	}
	dst := make(map[model.QName][]model.QName, len(src))
	for key, value := range src {
		dst[key] = slices.Clone(value)
	}
	return dst
}
