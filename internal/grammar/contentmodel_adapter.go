package grammar

import (
	"github.com/jacoelho/xsd/internal/grammar/contentmodel"
	"github.com/jacoelho/xsd/internal/types"
)

// BuildAutomaton builds an automaton from compiled particles.
// This function breaks the import cycle by converting CompiledParticle to ParticleAdapter.
// elementFormQualified should be true if elementFormDefault="qualified" in the schema.
func BuildAutomaton(particles []*CompiledParticle, targetNamespace types.NamespaceURI, elementFormQualified bool) (*contentmodel.Automaton, error) {
	adapters := make([]*contentmodel.ParticleAdapter, len(particles))
	for i, p := range particles {
		adapters[i] = convertParticle(p)
	}

	builder := contentmodel.NewBuilder(adapters, nil, string(targetNamespace), elementFormQualified)
	return builder.Build()
}

// convertParticle converts a CompiledParticle to a ParticleAdapter.
func convertParticle(p *CompiledParticle) *contentmodel.ParticleAdapter {
	adapter := &contentmodel.ParticleAdapter{
		Kind:              int(p.Kind),
		MinOccurs:         p.MinOccurs,
		MaxOccurs:         p.MaxOccurs,
		GroupKind:         p.GroupKind,
		Wildcard:          p.Wildcard,
		AllowSubstitution: p.IsReference,
	}

	if p.Element != nil {
		adapter.Element = p.Element
		if p.Element.Original != nil {
			adapter.Original = p.Element.Original
		}
	} else if p.Wildcard != nil {
		adapter.Original = p.Wildcard
	}

	if len(p.Children) > 0 {
		adapter.Children = make([]*contentmodel.ParticleAdapter, len(p.Children))
		for i, child := range p.Children {
			adapter.Children[i] = convertParticle(child)
		}
	}

	return adapter
}
