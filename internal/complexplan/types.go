package complexplan

import "github.com/jacoelho/xsd/internal/model"

// ComplexTypes exposes effective complex-type entries shared across phases.
type ComplexTypes struct {
	plan *plan
}

// Entry returns the effective entry for ct.
func (c *ComplexTypes) Entry(ct *model.ComplexType) (Entry, bool) {
	if c == nil || c.plan == nil {
		return Entry{}, false
	}
	return c.plan.entry(ct)
}

// AttributeUses returns precomputed effective attribute uses and wildcard for ct.
func (c *ComplexTypes) AttributeUses(ct *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, bool) {
	if c == nil || c.plan == nil {
		return nil, nil, false
	}
	return c.plan.attributeUses(ct)
}

// Content returns the precomputed effective content particle for ct.
func (c *ComplexTypes) Content(ct *model.ComplexType) (model.Particle, bool) {
	if c == nil || c.plan == nil {
		return nil, false
	}
	return c.plan.content(ct)
}

// SimpleContentType returns the precomputed simple-content text type for ct.
func (c *ComplexTypes) SimpleContentType(ct *model.ComplexType) (model.Type, bool) {
	if c == nil || c.plan == nil {
		return nil, false
	}
	return c.plan.simpleContentType(ct)
}

// Entry is the effective semantic view of one complex type.
type Entry struct {
	Content        model.Particle
	SimpleTextType model.Type
	Wildcard       *model.AnyAttribute
	Attributes     []*model.AttributeDecl
}
