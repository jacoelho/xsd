package loadmerge

import (
	"cmp"
	"maps"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

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
	clone.GlobalDecls = append([]parser.GlobalDecl(nil), sch.GlobalDecls...)
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
	if src == nil {
		return nil
	}
	dst := make(map[K]V, len(src))
	maps.Copy(dst, src)
	return dst
}

func sortedQNames[V any](m map[types.QName]V) []types.QName {
	keys := make([]types.QName, 0, len(m))
	for qname := range m {
		keys = append(keys, qname)
	}
	slices.SortFunc(keys, func(a, b types.QName) int {
		if a.Namespace != b.Namespace {
			return cmp.Compare(a.Namespace, b.Namespace)
		}
		return cmp.Compare(a.Local, b.Local)
	})
	return keys
}

func copyImportContexts(src map[string]parser.ImportContext) map[string]parser.ImportContext {
	if src == nil {
		return nil
	}
	dst := make(map[string]parser.ImportContext, len(src))
	for key, ctx := range src {
		copied := ctx
		if ctx.Imports != nil {
			imports := make(map[types.NamespaceURI]bool, len(ctx.Imports))
			for ns := range ctx.Imports {
				imports[ns] = true
			}
			copied.Imports = imports
		} else {
			copied.Imports = nil
		}
		dst[key] = copied
	}
	return dst
}

func copyImportedNamespaces(src map[types.NamespaceURI]map[types.NamespaceURI]bool) map[types.NamespaceURI]map[types.NamespaceURI]bool {
	if src == nil {
		return nil
	}
	dst := make(map[types.NamespaceURI]map[types.NamespaceURI]bool, len(src))
	for ns, imports := range src {
		if imports == nil {
			dst[ns] = nil
			continue
		}
		copied := make(map[types.NamespaceURI]bool, len(imports))
		for imported := range imports {
			copied[imported] = true
		}
		dst[ns] = copied
	}
	return dst
}

func copyQNameSliceMap(src map[types.QName][]types.QName) map[types.QName][]types.QName {
	if src == nil {
		return nil
	}
	dst := make(map[types.QName][]types.QName, len(src))
	for key, value := range src {
		if value == nil {
			dst[key] = nil
			continue
		}
		copied := make([]types.QName, len(value))
		copy(copied, value)
		dst[key] = copied
	}
	return dst
}
