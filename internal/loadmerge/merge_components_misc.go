package loadmerge

import (
	"github.com/jacoelho/xsd/internal/model"
	qnameorder "github.com/jacoelho/xsd/internal/qname"
)

func (c *mergeContext) mergeSubstitutionGroups() {
	if c.target.SubstitutionGroups == nil {
		c.target.SubstitutionGroups = make(map[model.QName][]model.QName)
	}
	heads := make([]model.QName, 0, len(c.source.SubstitutionGroups))
	for head := range c.source.SubstitutionGroups {
		heads = append(heads, head)
	}
	qnameorder.SortInPlace(heads)
	for _, head := range heads {
		members := c.source.SubstitutionGroups[head]
		targetHead := c.remapQName(head)
		remappedMembers := make([]model.QName, 0, len(members))
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

func sortAndDedupeQNames(names []model.QName) []model.QName {
	return qnameorder.SortAndDedupe(names)
}

func (c *mergeContext) mergeNotationDecls() error {
	return mergeNamed(
		c.source.NotationDecls,
		c.target.NotationDecls,
		c.target.NotationOrigins,
		c.remapQName,
		func(qname model.QName) string { return c.originFor(c.source.NotationOrigins, qname) },
		func(notation *model.NotationDecl) *model.NotationDecl { return notation.Copy(c.opts) },
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

func (c *mergeContext) originFor(origins map[model.QName]string, qname model.QName) string {
	origin := origins[qname]
	if origin == "" {
		origin = c.source.Location
	}
	return origin
}
