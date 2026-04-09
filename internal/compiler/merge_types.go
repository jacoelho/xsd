package compiler

import (
	"maps"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// Kind identifies how a parsed schema should be combined with another.
type Kind int

const (
	Include Kind = iota
	Import
)

// NamespaceMode controls chameleon-include remapping.
type NamespaceMode int

const (
	RemapNamespace NamespaceMode = iota
	KeepNamespace
)

type mergeContext struct {
	targetGraph                *parser.SchemaGraph
	targetMeta                 *parser.SchemaMeta
	sourceGraph                *parser.SchemaGraph
	sourceMeta                 *parser.SchemaMeta
	insertedGlobalDecls        [parser.GlobalDeclNotation + 1]insertedQNameSet
	expectedInsertCounts       [parser.GlobalDeclNotation + 1]int
	expectedInsertCountsCached [parser.GlobalDeclNotation + 1]bool
	remapQName                 func(model.QName) model.QName
	opts                       model.CopyOptions
	elementDeclsOwned          bool
	typeDefsOwned              bool
	attributeDeclsOwned        bool
	attributeGroupsOwned       bool
	groupsOwned                bool
	notationDeclsOwned         bool
	elementOriginsOwned        bool
	typeOriginsOwned           bool
	attributeOriginsOwned      bool
	attributeGroupOriginsOwned bool
	groupOriginsOwned          bool
	notationOriginsOwned       bool
	isImport                   bool
	needsNamespaceRemap        bool
}

func newMergeContext(target, source *parser.Schema, kind Kind, remap NamespaceMode) mergeContext {
	isImport := kind == Import
	needsNamespaceRemap := remap == RemapNamespace
	remapQName := func(qname model.QName) model.QName {
		if needsNamespaceRemap && qname.Namespace == "" {
			return model.QName{
				Namespace: target.TargetNamespace,
				Local:     qname.Local,
			}
		}
		return qname
	}

	sourceNamespace := source.TargetNamespace
	if !isImport && needsNamespaceRemap {
		sourceNamespace = target.TargetNamespace
	}

	opts := model.CopyOptions{
		SourceNamespace: sourceNamespace,
		RemapQName:      remapQName,
	}
	opts = model.WithGraphMemo(opts)

	return mergeContext{
		targetGraph:         &target.SchemaGraph,
		targetMeta:          &target.SchemaMeta,
		sourceGraph:         &source.SchemaGraph,
		sourceMeta:          &source.SchemaMeta,
		isImport:            isImport,
		needsNamespaceRemap: needsNamespaceRemap,
		remapQName:          remapQName,
		opts:                opts,
	}
}

func expectedQNameInsertCount[V any](source map[model.QName]V, target map[model.QName]V, remap func(model.QName) model.QName) int {
	if len(source) == 0 {
		return 0
	}
	count := 0
	for name := range source {
		if _, exists := target[remap(name)]; exists {
			continue
		}
		count++
	}
	return count
}

func ensureOwnedQNameMap[V any](owned *bool, target *map[model.QName]V, extraCap int) map[model.QName]V {
	if target == nil {
		return nil
	}
	if extraCap < 0 {
		extraCap = 0
	}
	if *target == nil {
		*target = make(map[model.QName]V, extraCap)
		*owned = true
		return *target
	}
	if !*owned {
		cloned := make(map[model.QName]V, len(*target)+extraCap)
		maps.Copy(cloned, *target)
		*target = cloned
		*owned = true
	}
	return *target
}

func (c *mergeContext) elementDeclsForInsert() map[model.QName]*model.ElementDecl {
	extra := 0
	if !c.elementDeclsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclElement)
	}
	return ensureOwnedQNameMap(&c.elementDeclsOwned, &c.targetGraph.ElementDecls, extra)
}

func (c *mergeContext) typeDefsForInsert() map[model.QName]model.Type {
	extra := 0
	if !c.typeDefsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclType)
	}
	return ensureOwnedQNameMap(&c.typeDefsOwned, &c.targetGraph.TypeDefs, extra)
}

func (c *mergeContext) attributeDeclsForInsert() map[model.QName]*model.AttributeDecl {
	extra := 0
	if !c.attributeDeclsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclAttribute)
	}
	return ensureOwnedQNameMap(&c.attributeDeclsOwned, &c.targetGraph.AttributeDecls, extra)
}

func (c *mergeContext) attributeGroupsForInsert() map[model.QName]*model.AttributeGroup {
	extra := 0
	if !c.attributeGroupsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclAttributeGroup)
	}
	return ensureOwnedQNameMap(&c.attributeGroupsOwned, &c.targetGraph.AttributeGroups, extra)
}

func (c *mergeContext) groupsForInsert() map[model.QName]*model.ModelGroup {
	extra := 0
	if !c.groupsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclGroup)
	}
	return ensureOwnedQNameMap(&c.groupsOwned, &c.targetGraph.Groups, extra)
}

func (c *mergeContext) notationDeclsForInsert() map[model.QName]*model.NotationDecl {
	extra := 0
	if !c.notationDeclsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclNotation)
	}
	return ensureOwnedQNameMap(&c.notationDeclsOwned, &c.targetGraph.NotationDecls, extra)
}

func (c *mergeContext) elementOriginsForInsert() map[model.QName]string {
	extra := 0
	if !c.elementOriginsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclElement)
	}
	return ensureOwnedQNameMap(&c.elementOriginsOwned, &c.targetMeta.ElementOrigins, extra)
}

func (c *mergeContext) typeOriginsForInsert() map[model.QName]string {
	extra := 0
	if !c.typeOriginsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclType)
	}
	return ensureOwnedQNameMap(&c.typeOriginsOwned, &c.targetMeta.TypeOrigins, extra)
}

func (c *mergeContext) attributeOriginsForInsert() map[model.QName]string {
	extra := 0
	if !c.attributeOriginsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclAttribute)
	}
	return ensureOwnedQNameMap(&c.attributeOriginsOwned, &c.targetMeta.AttributeOrigins, extra)
}

func (c *mergeContext) attributeGroupOriginsForInsert() map[model.QName]string {
	extra := 0
	if !c.attributeGroupOriginsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclAttributeGroup)
	}
	return ensureOwnedQNameMap(&c.attributeGroupOriginsOwned, &c.targetMeta.AttributeGroupOrigins, extra)
}

func (c *mergeContext) groupOriginsForInsert() map[model.QName]string {
	extra := 0
	if !c.groupOriginsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclGroup)
	}
	return ensureOwnedQNameMap(&c.groupOriginsOwned, &c.targetMeta.GroupOrigins, extra)
}

func (c *mergeContext) notationOriginsForInsert() map[model.QName]string {
	extra := 0
	if !c.notationOriginsOwned {
		extra = c.expectedInsertedGlobalDeclCount(parser.GlobalDeclNotation)
	}
	return ensureOwnedQNameMap(&c.notationOriginsOwned, &c.targetMeta.NotationOrigins, extra)
}
