package complextypeplan

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
)

type ComputeFuncs struct {
	AttributeUses     func(*model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, error)
	ContentParticle   func(*model.ComplexType) model.Particle
	SimpleContentType func(*model.ComplexType) (model.Type, error)
}

type Entry struct {
	Attributes      []*model.AttributeDecl
	Wildcard        *model.AnyAttribute
	ContentParticle model.Particle
	SimpleTextType  model.Type
}

// Plan stores precomputed complex-type artifacts for reuse across compile/build phases.
type Plan struct {
	entries map[*model.ComplexType]Entry
}

// Build computes immutable per-complex-type artifacts in registry order.
func Build(registry *schema.Registry, funcs ComputeFuncs) (*Plan, error) {
	if registry == nil {
		return nil, fmt.Errorf("complex type plan: registry is nil")
	}
	entries := make(map[*model.ComplexType]Entry, len(registry.TypeOrder))
	for _, typeEntry := range registry.TypeOrder {
		ct, ok := model.AsComplexType(typeEntry.Type)
		if !ok || ct == nil {
			continue
		}
		entry := Entry{}
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
	return &Plan{entries: entries}, nil
}

// Entry returns the precomputed entry for ct.
func (p *Plan) Entry(ct *model.ComplexType) (Entry, bool) {
	if p == nil || ct == nil {
		return Entry{}, false
	}
	entry, ok := p.entries[ct]
	return entry, ok
}

// AttributeUses returns precomputed effective attribute uses and wildcard for ct.
func (p *Plan) AttributeUses(ct *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, bool) {
	entry, ok := p.Entry(ct)
	if !ok {
		return nil, nil, false
	}
	return entry.Attributes, entry.Wildcard, true
}

// Content returns precomputed effective content particle for ct.
func (p *Plan) Content(ct *model.ComplexType) (model.Particle, bool) {
	entry, ok := p.Entry(ct)
	if !ok {
		return nil, false
	}
	return entry.ContentParticle, true
}

// SimpleContentType returns precomputed simple-content text type for ct.
func (p *Plan) SimpleContentType(ct *model.ComplexType) (model.Type, bool) {
	entry, ok := p.Entry(ct)
	if !ok {
		return nil, false
	}
	return entry.SimpleTextType, true
}
