package complexplan

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
)

// BuildFuncs groups callbacks used to precompute complex-type artifacts.
type BuildFuncs struct {
	AttributeUses     func(*model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, error)
	ContentParticle   func(*model.ComplexType) model.Particle
	SimpleContentType func(*model.ComplexType) (model.Type, error)
}

// planEntry stores precomputed artifacts for a single complex type.
type planEntry struct {
	ContentParticle model.Particle
	SimpleTextType  model.Type
	Wildcard        *model.AnyAttribute
	Attributes      []*model.AttributeDecl
}

// plan stores precomputed complex-type artifacts for reuse across compile/build phases.
type plan struct {
	entries map[*model.ComplexType]planEntry
}

// Build computes immutable per-complex-type artifacts in registry order.
func Build(registry *analysis.Registry, funcs BuildFuncs) (*ComplexTypes, error) {
	if registry == nil {
		return nil, fmt.Errorf("complex type plan: registry is nil")
	}
	entries := make(map[*model.ComplexType]planEntry, len(registry.TypeOrder))
	for _, typeEntry := range registry.TypeOrder {
		ct, ok := model.AsComplexType(typeEntry.Type)
		if !ok || ct == nil {
			continue
		}
		entry := planEntry{}
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
	return &ComplexTypes{plan: &plan{entries: entries}}, nil
}

// entry returns the precomputed entry for ct.
func (p *plan) entry(ct *model.ComplexType) (Entry, bool) {
	if p == nil || ct == nil {
		return Entry{}, false
	}
	entry, ok := p.entries[ct]
	if !ok {
		return Entry{}, false
	}
	return Entry{
		Content:        entry.ContentParticle,
		Attributes:     entry.Attributes,
		Wildcard:       entry.Wildcard,
		SimpleTextType: entry.SimpleTextType,
	}, true
}

// attributeUses returns precomputed effective attribute uses and wildcard for ct.
func (p *plan) attributeUses(ct *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, bool) {
	entry, ok := p.entry(ct)
	if !ok {
		return nil, nil, false
	}
	return entry.Attributes, entry.Wildcard, true
}

// content returns precomputed effective content particle for ct.
func (p *plan) content(ct *model.ComplexType) (model.Particle, bool) {
	entry, ok := p.entry(ct)
	if !ok {
		return nil, false
	}
	return entry.Content, true
}

// simpleContentType returns precomputed simple-content text type for ct.
func (p *plan) simpleContentType(ct *model.ComplexType) (model.Type, bool) {
	entry, ok := p.entry(ct)
	if !ok {
		return nil, false
	}
	return entry.SimpleTextType, true
}
