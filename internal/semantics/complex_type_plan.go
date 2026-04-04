package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
)

// ComplexTypePlanFuncs groups callbacks used to precompute complex-type artifacts.
type ComplexTypePlanFuncs struct {
	AttributeUses     func(*model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, error)
	ContentParticle   func(*model.ComplexType) model.Particle
	SimpleContentType func(*model.ComplexType) (model.Type, error)
}

// ComplexTypePlanEntry stores precomputed artifacts for a single complex type.
type ComplexTypePlanEntry struct {
	ContentParticle model.Particle
	SimpleTextType  model.Type
	Wildcard        *model.AnyAttribute
	Attributes      []*model.AttributeDecl
}

// ComplexTypePlan stores precomputed complex-type artifacts for reuse across compile/build phases.
type ComplexTypePlan struct {
	entries map[*model.ComplexType]ComplexTypePlanEntry
}

// buildComplexTypePlanEntries computes immutable per-complex-type artifacts in registry order.
func buildComplexTypePlanEntries(registry *analysis.Registry, funcs ComplexTypePlanFuncs) (*ComplexTypePlan, error) {
	if registry == nil {
		return nil, fmt.Errorf("complex type plan: registry is nil")
	}
	entries := make(map[*model.ComplexType]ComplexTypePlanEntry, len(registry.TypeOrder))
	for _, typeEntry := range registry.TypeOrder {
		ct, ok := model.AsComplexType(typeEntry.Type)
		if !ok || ct == nil {
			continue
		}
		entry := ComplexTypePlanEntry{}
		if funcs.AttributeUses != nil {
			attrs, wildcard, err := funcs.AttributeUses(ct)
			if err != nil {
				return nil, err
			}
			entry.Attributes = append(entry.Attributes, attrs...)
			entry.Wildcard = wildcard
		}
		if funcs.ContentParticle != nil {
			entry.ContentParticle = funcs.ContentParticle(ct)
		}
		if funcs.SimpleContentType != nil {
			textType, err := funcs.SimpleContentType(ct)
			if err != nil {
				return nil, err
			}
			entry.SimpleTextType = textType
		}
		entries[ct] = entry
	}
	return &ComplexTypePlan{entries: entries}, nil
}

// Entry returns the precomputed entry for ct.
func (p *ComplexTypePlan) Entry(ct *model.ComplexType) (ComplexTypePlanEntry, bool) {
	if p == nil || ct == nil {
		return ComplexTypePlanEntry{}, false
	}
	entry, ok := p.entries[ct]
	return entry, ok
}

// AttributeUses returns precomputed effective attribute uses and wildcard for ct.
func (p *ComplexTypePlan) AttributeUses(ct *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, bool) {
	entry, ok := p.Entry(ct)
	if !ok {
		return nil, nil, false
	}
	return entry.Attributes, entry.Wildcard, true
}

// Content returns precomputed effective content particle for ct.
func (p *ComplexTypePlan) Content(ct *model.ComplexType) (model.Particle, bool) {
	entry, ok := p.Entry(ct)
	if !ok {
		return nil, false
	}
	return entry.ContentParticle, true
}

// SimpleContentType returns precomputed simple-content text type for ct.
func (p *ComplexTypePlan) SimpleContentType(ct *model.ComplexType) (model.Type, bool) {
	entry, ok := p.Entry(ct)
	if !ok {
		return nil, false
	}
	return entry.SimpleTextType, true
}
