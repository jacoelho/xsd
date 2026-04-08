package compiler

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func (c *mergeContext) mergeSubstitutionGroups() {
	if c.targetGraph.SubstitutionGroups == nil {
		c.targetGraph.SubstitutionGroups = make(map[model.QName][]model.QName)
	}
	heads := model.SortedMapKeys(c.sourceGraph.SubstitutionGroups)
	for _, head := range heads {
		members := c.sourceGraph.SubstitutionGroups[head]
		targetHead := c.remapQName(head)
		remappedMembers := make([]model.QName, 0, len(members))
		for _, member := range members {
			remapped := c.remapQName(member)
			remappedMembers = append(remappedMembers, remapped)
		}
		remappedMembers = sortAndDedupeQNames(remappedMembers)
		if existing, exists := c.targetGraph.SubstitutionGroups[targetHead]; exists {
			if len(remappedMembers) == 0 {
				c.targetGraph.SubstitutionGroups[targetHead] = sortAndDedupeQNames(existing)
				continue
			}
			existing = append(existing, remappedMembers...)
			c.targetGraph.SubstitutionGroups[targetHead] = sortAndDedupeQNames(existing)
			continue
		}
		if len(remappedMembers) > 0 {
			c.targetGraph.SubstitutionGroups[targetHead] = remappedMembers
		}
	}
}

func sortAndDedupeQNames(names []model.QName) []model.QName {
	return model.SortAndDedupe(names)
}

func (c *mergeContext) mergeNotationDecls() error {
	return mergeNamedGlobalDecl(
		c,
		parser.GlobalDeclNotation,
		c.sourceGraph.NotationDecls,
		c.targetGraph.NotationDecls,
		c.targetMeta.NotationOrigins,
		c.notationDeclsForInsert,
		c.notationOriginsForInsert,
		c.sourceMeta.NotationOrigins,
		func(notation *model.NotationDecl) *model.NotationDecl { return notation.Copy(c.opts) },
		"notation",
	)
}

func (c *mergeContext) mergeIDAttributes() error {
	if c.targetMeta.IDAttributes == nil {
		c.targetMeta.IDAttributes = make(map[string]string)
	}
	for id, component := range c.sourceMeta.IDAttributes {
		if _, exists := c.targetMeta.IDAttributes[id]; exists {
			continue
		}
		c.targetMeta.IDAttributes[id] = component
	}
	return nil
}

func (c *mergeContext) originFor(origins map[model.QName]string, name model.QName) string {
	origin := origins[name]
	if origin == "" {
		origin = c.sourceMeta.Location
	}
	return origin
}
