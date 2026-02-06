package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

// GetContentParticle extracts the particle from any content type.
// Returns nil if content doesn't contain a particle.
func GetContentParticle(content types.Content) types.Particle {
	return traversal.GetContentParticle(content)
}

// WalkContentParticles visits all particles in content (extension + restriction).
// Calls fn for each particle found. Returns the first error encountered.
func WalkContentParticles(content types.Content, fn func(types.Particle) error) error {
	return traversal.WalkContentParticles(content, fn)
}

// WalkParticles recursively visits all particles in a tree.
// Calls fn for each particle found, including nested particles in ModelGroups.
// Returns the first error encountered.
func WalkParticles(particle types.Particle, fn func(types.Particle) error) error {
	return traversal.WalkParticles(particle, fn)
}

// CollectElements returns all ElementDecl in a particle tree.
func CollectElements(particle types.Particle) []*types.ElementDecl {
	return traversal.CollectElements(particle)
}

// CollectWildcards returns all AnyElement in a particle tree.
func CollectWildcards(particle types.Particle) []*types.AnyElement {
	return traversal.CollectWildcards(particle)
}
