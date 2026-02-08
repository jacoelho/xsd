package loadmerge

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *mergeContext) mergeImportedNamespaces() {
	if c.source.ImportedNamespaces == nil {
		return
	}
	if c.target.ImportedNamespaces == nil {
		c.target.ImportedNamespaces = make(map[types.NamespaceURI]map[types.NamespaceURI]bool)
	}
	for fromNS, imports := range c.source.ImportedNamespaces {
		mappedFrom := fromNS
		if c.needsNamespaceRemap && fromNS.IsEmpty() {
			mappedFrom = c.target.TargetNamespace
		}
		if _, ok := c.target.ImportedNamespaces[mappedFrom]; !ok {
			c.target.ImportedNamespaces[mappedFrom] = make(map[types.NamespaceURI]bool)
		}
		for ns := range imports {
			c.target.ImportedNamespaces[mappedFrom][ns] = true
		}
	}
}

func (c *mergeContext) mergeImportContexts() {
	if c.source.ImportContexts == nil {
		return
	}
	if c.target.ImportContexts == nil {
		c.target.ImportContexts = make(map[string]parser.ImportContext)
	}
	for location, ctx := range c.source.ImportContexts {
		merged := ctx
		if c.needsNamespaceRemap && merged.TargetNamespace.IsEmpty() {
			merged.TargetNamespace = c.target.TargetNamespace
		}
		if merged.Imports == nil {
			merged.Imports = make(map[types.NamespaceURI]bool)
		}
		if existing, ok := c.target.ImportContexts[location]; ok {
			if existing.Imports == nil {
				existing.Imports = make(map[types.NamespaceURI]bool)
			}
			for ns := range merged.Imports {
				existing.Imports[ns] = true
			}
			if existing.TargetNamespace.IsEmpty() {
				existing.TargetNamespace = merged.TargetNamespace
			}
			c.target.ImportContexts[location] = existing
			continue
		}
		c.target.ImportContexts[location] = merged
	}
}
