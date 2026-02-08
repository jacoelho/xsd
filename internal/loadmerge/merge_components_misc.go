package loadmerge

import (
	"slices"

	"github.com/jacoelho/xsd/internal/types"
)

func (c *mergeContext) mergeSubstitutionGroups() {
	if c.target.SubstitutionGroups == nil {
		c.target.SubstitutionGroups = make(map[types.QName][]types.QName)
	}
	heads := make([]types.QName, 0, len(c.source.SubstitutionGroups))
	for head := range c.source.SubstitutionGroups {
		heads = append(heads, head)
	}
	slices.SortFunc(heads, types.CompareQName)
	for _, head := range heads {
		members := c.source.SubstitutionGroups[head]
		targetHead := c.remapQName(head)
		remappedMembers := make([]types.QName, 0, len(members))
		for _, member := range members {
			remapped := c.remapQName(member)
			remappedMembers = append(remappedMembers, remapped)
		}
		remappedMembers = sortAndDedupeQNames(remappedMembers)
		if existing, exists := c.target.SubstitutionGroups[targetHead]; exists {
			if len(remappedMembers) == 0 {
				c.target.SubstitutionGroups[targetHead] = sortAndDedupeQNames(existing)
				continue
			}
			existing = append(existing, remappedMembers...)
			c.target.SubstitutionGroups[targetHead] = sortAndDedupeQNames(existing)
			continue
		}
		if len(remappedMembers) > 0 {
			c.target.SubstitutionGroups[targetHead] = remappedMembers
		}
	}
}

func sortAndDedupeQNames(names []types.QName) []types.QName {
	if len(names) < 2 {
		return names
	}
	slices.SortFunc(names, types.CompareQName)
	out := names[:0]
	var last types.QName
	for i, name := range names {
		if i == 0 || !name.Equal(last) {
			out = append(out, name)
			last = name
		}
	}
	return out
}

func (c *mergeContext) mergeNotationDecls() error {
	return mergeNamed(
		c.source.NotationDecls,
		c.target.NotationDecls,
		c.target.NotationOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.NotationOrigins, qname) },
		func(notation *types.NotationDecl) *types.NotationDecl { return notation.Copy(c.opts) },
		nil,
		nil,
		"notation",
	)
}

func (c *mergeContext) mergeIDAttributes() error {
	if c.target.IDAttributes == nil {
		c.target.IDAttributes = make(map[string]string)
	}
	for id, component := range c.source.IDAttributes {
		if _, exists := c.target.IDAttributes[id]; exists {
			continue
		}
		c.target.IDAttributes[id] = component
	}
	return nil
}

func (c *mergeContext) originFor(origins map[types.QName]string, qname types.QName) string {
	origin := origins[qname]
	if origin == "" {
		origin = c.source.Location
	}
	return origin
}
