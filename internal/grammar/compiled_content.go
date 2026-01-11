package grammar

import "github.com/jacoelho/xsd/internal/types"

// ParticleKind classifies compiled particles.
type ParticleKind int

const (
	// ParticleElement represents an element particle.
	ParticleElement ParticleKind = iota
	// ParticleGroup represents a model group particle.
	ParticleGroup
	// ParticleWildcard represents an xs:any wildcard particle.
	ParticleWildcard
)

// CompiledContentModel is a pre-compiled content model.
// All group references are expanded. The automaton is pre-built for O(n) schemacheck.
type CompiledContentModel struct {
	Automaton        *Automaton
	ElementIndex     map[types.QName]*CompiledElement
	Particles        []*CompiledParticle
	AllElements      []*AllGroupElement
	SimpleSequence   []*CompiledParticle
	Kind             types.GroupKind
	MinOccurs        int
	Empty            bool
	RejectAll        bool
	Mixed            bool
	IsSimpleSequence bool
}

// AllGroupElement represents an element in an all group.
// Implements AllGroupElementInfo interface.
// Note: Elements with maxOccurs=0 are filtered out during compilation per XSD spec.
type AllGroupElement struct {
	Element *CompiledElement
	// true if minOccurs=0
	Optional bool
	// true if this element is a ref="..."
	AllowSubstitution bool
}

// ElementQName returns the QName of the element.
func (e *AllGroupElement) ElementQName() types.QName {
	if e.Element == nil {
		return types.QName{}
	}
	if !e.Element.EffectiveQName.IsZero() {
		return e.Element.EffectiveQName
	}
	return e.Element.QName
}

// ElementDecl returns the compiled element for this all-group entry.
func (e *AllGroupElement) ElementDecl() *CompiledElement {
	if e == nil {
		return nil
	}
	return e.Element
}

// IsOptional returns true if minOccurs=0.
func (e *AllGroupElement) IsOptional() bool {
	return e.Optional
}

// AllowsSubstitution returns true if substitution groups apply to this element.
func (e *AllGroupElement) AllowsSubstitution() bool {
	return e.AllowSubstitution
}

// CompiledParticle is a particle with resolved element type.
type CompiledParticle struct {
	Element     *CompiledElement
	Wildcard    *types.AnyElement
	Children    []*CompiledParticle
	Kind        ParticleKind
	MinOccurs   int
	MaxOccurs   int
	GroupKind   types.GroupKind
	IsReference bool
}

// Wildcards returns all wildcards in the content model.
func (cm *CompiledContentModel) Wildcards() []*types.AnyElement {
	return collectWildcards(cm.Particles)
}

// collectWildcards recursively collects wildcards from particles.
func collectWildcards(particles []*CompiledParticle) []*types.AnyElement {
	var wildcards []*types.AnyElement
	for _, p := range particles {
		switch p.Kind {
		case ParticleWildcard:
			if p.Wildcard != nil {
				wildcards = append(wildcards, p.Wildcard)
			}
		case ParticleGroup:
			wildcards = append(wildcards, collectWildcards(p.Children)...)
		}
	}
	return wildcards
}
