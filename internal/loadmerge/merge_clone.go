package loadmerge

import (
	"maps"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	qnameorder "github.com/jacoelho/xsd/internal/qname"
)

// CloneSchemaForMerge is an exported function.
func CloneSchemaForMerge(sch *parser.Schema) *parser.Schema {
	// include/import merging runs during schema compilation only, so a full
	// defensive clone is preferred over aliasing mutable maps across documents.
	clone := *sch
	clone.ImportContexts = copyImportContexts(sch.ImportContexts)
	clone.ImportedNamespaces = copyImportedNamespaces(sch.ImportedNamespaces)
	clone.ElementDecls = cloneMap(sch.ElementDecls)
	clone.ElementOrigins = cloneMap(sch.ElementOrigins)
	clone.TypeDefs = cloneMap(sch.TypeDefs)
	clone.TypeOrigins = cloneMap(sch.TypeOrigins)
	clone.AttributeDecls = cloneMap(sch.AttributeDecls)
	clone.AttributeOrigins = cloneMap(sch.AttributeOrigins)
	clone.AttributeGroups = cloneMap(sch.AttributeGroups)
	clone.AttributeGroupOrigins = cloneMap(sch.AttributeGroupOrigins)
	clone.Groups = cloneMap(sch.Groups)
	clone.GroupOrigins = cloneMap(sch.GroupOrigins)
	clone.SubstitutionGroups = copyQNameSliceMap(sch.SubstitutionGroups)
	clone.NotationDecls = cloneMap(sch.NotationDecls)
	clone.NotationOrigins = cloneMap(sch.NotationOrigins)
	clone.IDAttributes = cloneMap(sch.IDAttributes)
	clone.GlobalDecls = slices.Clone(sch.GlobalDecls)
	return &clone
}

// CloneSchemaDeep clones a parsed schema and its mutable declaration graph.
func CloneSchemaDeep(sch *parser.Schema) (*parser.Schema, error) {
	if sch == nil {
		return nil, nil
	}
	clone := parser.NewSchema()
	clone.Location = sch.Location
	clone.TargetNamespace = sch.TargetNamespace
	clone.AttributeFormDefault = sch.AttributeFormDefault
	clone.ElementFormDefault = sch.ElementFormDefault
	clone.BlockDefault = sch.BlockDefault
	clone.FinalDefault = sch.FinalDefault
	clone.NamespaceDecls = cloneMap(sch.NamespaceDecls)
	ctx := newMergeContext(clone, sch, MergeInclude, KeepNamespace)
	ctx.opts.PreserveSourceNamespace = true
	existingDecls := existingGlobalDecls(clone)
	ctx.mergeImportedNamespaces()
	ctx.mergeImportContexts()
	if err := ctx.mergeElementDecls(); err != nil {
		return nil, err
	}
	if err := ctx.mergeTypeDefs(); err != nil {
		return nil, err
	}
	if err := ctx.mergeAttributeDecls(); err != nil {
		return nil, err
	}
	if err := ctx.mergeAttributeGroups(); err != nil {
		return nil, err
	}
	if err := ctx.mergeGroups(); err != nil {
		return nil, err
	}
	ctx.mergeSubstitutionGroups()
	if err := ctx.mergeNotationDecls(); err != nil {
		return nil, err
	}
	if err := ctx.mergeIDAttributes(); err != nil {
		return nil, err
	}
	ctx.mergeGlobalDecls(existingDecls, 0)
	return clone, nil
}

func cloneMap[K comparable, V any](src map[K]V) map[K]V {
	return maps.Clone(src)
}

func sortedQNames[V any](m map[model.QName]V) []model.QName {
	return qnameorder.SortedMapKeys(m)
}

func copyImportContexts(src map[string]parser.ImportContext) map[string]parser.ImportContext {
	if src == nil {
		return nil
	}
	dst := make(map[string]parser.ImportContext, len(src))
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
