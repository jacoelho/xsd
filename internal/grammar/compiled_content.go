package grammar

import (
	"github.com/jacoelho/xsd/internal/grammar/contentmodel"
	"github.com/jacoelho/xsd/internal/types"
)

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
// All group references are expanded. The automaton is pre-built for O(n) validation.
type CompiledContentModel struct {
	Kind      types.GroupKind         // Sequence, Choice, All
	Particles []*CompiledParticle     // Flattened (no GroupRefs)
	Automaton *contentmodel.Automaton // Pre-compiled DFA (not used for AllGroup)
	Empty     bool                    // True if content can be empty
	RejectAll bool                    // True if content model accepts no instances
	Mixed     bool                    // True if mixed content allowed
	MinOccurs int                     // MinOccurs of the top-level group (default 1)

	// For AllGroup - uses simple array-based validation instead of DFA
	AllElements []*AllGroupElement // Elements in the all group

	// Cached validation data (precomputed during compilation)
	ElementIndex     map[types.QName]*CompiledElement
	SimpleSequence   []*CompiledParticle
	IsSimpleSequence bool
}

// AllGroupElement represents an element in an all group.
// Implements contentmodel.AllGroupElementInfo interface.
// Note: Elements with maxOccurs=0 are filtered out during compilation per XSD spec.
type AllGroupElement struct {
	Element           *CompiledElement
	Optional          bool // true if minOccurs=0
	AllowSubstitution bool // true if this element is a ref="..."
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
func (e *AllGroupElement) ElementDecl() any {
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
	Kind      ParticleKind
	MinOccurs int
	MaxOccurs int
	// IsReference is true when this particle is from a ref="..." element.
	// Substitution groups are only allowed for references.
	IsReference bool

	// For element particles
	Element *CompiledElement

	// For group particles (sequence/choice/all)
	Children  []*CompiledParticle
	GroupKind types.GroupKind

	// For wildcard particles
	Wildcard *types.AnyElement
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
