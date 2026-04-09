package parser

import (
	"maps"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
)

// CloneSchemaForMerge returns a schema copy suitable for in-place merge staging.
func CloneSchemaForMerge(sch *Schema) *Schema {
	if sch == nil {
		return nil
	}
	return &Schema{
		SchemaGraph: cloneSchemaGraphForMerge(sch.SchemaGraph),
		SchemaMeta:  cloneSchemaMeta(sch.SchemaMeta),
	}
}

// CloneSchema deep-clones a parsed schema and its mutable declaration graph.
func CloneSchema(sch *Schema) *Schema {
	if sch == nil {
		return nil
	}

	clone := &Schema{
		SchemaGraph: newSchemaGraph(),
		SchemaMeta:  cloneSchemaMeta(sch.SchemaMeta),
	}
	clone.GlobalDecls = slices.Clone(sch.GlobalDecls)
	clone.SubstitutionGroups = copyQNameSliceMap(sch.SubstitutionGroups)

	opts := model.WithGraphMemo(model.CopyOptions{
		RemapQName:              model.NilRemap,
		SourceNamespace:         sch.TargetNamespace,
		PreserveSourceNamespace: true,
	})

	for _, name := range model.SortedMapKeys(sch.ElementDecls) {
		clone.ElementDecls[name] = sch.ElementDecls[name].Copy(opts)
	}
	maps.Copy(clone.ElementOrigins, sch.ElementOrigins)

	for _, name := range model.SortedMapKeys(sch.TypeDefs) {
		clone.TypeDefs[name] = model.CopyType(sch.TypeDefs[name], opts)
	}
	maps.Copy(clone.TypeOrigins, sch.TypeOrigins)

	for _, name := range model.SortedMapKeys(sch.AttributeDecls) {
		clone.AttributeDecls[name] = sch.AttributeDecls[name].Copy(opts)
	}
	maps.Copy(clone.AttributeOrigins, sch.AttributeOrigins)

	for _, name := range model.SortedMapKeys(sch.AttributeGroups) {
		clone.AttributeGroups[name] = sch.AttributeGroups[name].Copy(opts)
	}
	maps.Copy(clone.AttributeGroupOrigins, sch.AttributeGroupOrigins)

	for _, name := range model.SortedMapKeys(sch.Groups) {
		clone.Groups[name] = sch.Groups[name].Copy(opts)
	}
	maps.Copy(clone.GroupOrigins, sch.GroupOrigins)

	for _, name := range model.SortedMapKeys(sch.NotationDecls) {
		clone.NotationDecls[name] = sch.NotationDecls[name].Copy(opts)
	}
	maps.Copy(clone.NotationOrigins, sch.NotationOrigins)

	return clone
}

func cloneSchemaGraphForMerge(src SchemaGraph) SchemaGraph {
	return SchemaGraph{
		// Merge staging only mutates declaration maps when a merge inserts new entries.
		// Keep them shared here and let the compiler take a copy on first write.
		Groups:             src.Groups,
		TypeDefs:           src.TypeDefs,
		AttributeDecls:     src.AttributeDecls,
		SubstitutionGroups: copyQNameSliceMap(src.SubstitutionGroups),
		AttributeGroups:    src.AttributeGroups,
		ElementDecls:       src.ElementDecls,
		NotationDecls:      src.NotationDecls,
		// Merge staging only needs copy-on-write semantics for GlobalDecls.
		GlobalDecls: src.GlobalDecls,
	}
}

func cloneSchemaMeta(src SchemaMeta) SchemaMeta {
	return SchemaMeta{
		ImportContexts: copyImportContexts(src.ImportContexts),
		// Merge staging only mutates origin maps when new declarations are inserted.
		// Keep them shared here and let the compiler take a copy on first write.
		ElementOrigins:        src.ElementOrigins,
		TypeOrigins:           src.TypeOrigins,
		AttributeOrigins:      src.AttributeOrigins,
		AttributeGroupOrigins: src.AttributeGroupOrigins,
		ImportedNamespaces:    copyImportedNamespaces(src.ImportedNamespaces),
		GroupOrigins:          src.GroupOrigins,
		NotationOrigins:       src.NotationOrigins,
		IDAttributes:          maps.Clone(src.IDAttributes),
		NamespaceDecls:        maps.Clone(src.NamespaceDecls),
		Location:              src.Location,
		TargetNamespace:       src.TargetNamespace,
		FinalDefault:          src.FinalDefault,
		AttributeFormDefault:  src.AttributeFormDefault,
		ElementFormDefault:    src.ElementFormDefault,
		BlockDefault:          src.BlockDefault,
	}
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
