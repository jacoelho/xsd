package merge

import (
	"maps"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func (c *mergeContext) mergeImportedNamespaces() {
	if c.sourceMeta.ImportedNamespaces == nil {
		return
	}
	if c.targetMeta.ImportedNamespaces == nil {
		c.targetMeta.ImportedNamespaces = make(map[model.NamespaceURI]map[model.NamespaceURI]bool)
	}
	for fromNS, imports := range c.sourceMeta.ImportedNamespaces {
		mappedFrom := fromNS
		if c.needsNamespaceRemap && fromNS == "" {
			mappedFrom = c.targetMeta.TargetNamespace
		}
		if _, ok := c.targetMeta.ImportedNamespaces[mappedFrom]; !ok {
			c.targetMeta.ImportedNamespaces[mappedFrom] = make(map[model.NamespaceURI]bool)
		}
		maps.Copy(c.targetMeta.ImportedNamespaces[mappedFrom], imports)
	}
}

func (c *mergeContext) mergeImportContexts() {
	if c.sourceMeta.ImportContexts == nil {
		return
	}
	if c.targetMeta.ImportContexts == nil {
		c.targetMeta.ImportContexts = make(map[string]parser.ImportContext)
	}
	for location, ctx := range c.sourceMeta.ImportContexts {
		merged := ctx
		if c.needsNamespaceRemap && merged.TargetNamespace == "" {
			merged.TargetNamespace = c.targetMeta.TargetNamespace
		}
		if merged.Imports == nil {
			merged.Imports = make(map[model.NamespaceURI]bool)
		}
		if existing, ok := c.targetMeta.ImportContexts[location]; ok {
			if existing.Imports == nil {
				existing.Imports = make(map[model.NamespaceURI]bool)
			}
			maps.Copy(existing.Imports, merged.Imports)
			if existing.TargetNamespace == "" {
				existing.TargetNamespace = merged.TargetNamespace
			}
			c.targetMeta.ImportContexts[location] = existing
			continue
		}
		merged.Imports = maps.Clone(merged.Imports)
		if merged.Imports == nil {
			merged.Imports = make(map[model.NamespaceURI]bool)
		}
		c.targetMeta.ImportContexts[location] = merged
	}
}
