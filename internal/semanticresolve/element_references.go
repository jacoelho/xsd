package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

// collectElementReferences collects element references from content models.
func collectElementReferences(content types.Content) []*types.ElementDecl {
	return traversal.CollectFromContent(content, func(p types.Particle) (*types.ElementDecl, bool) {
		decl, ok := p.(*types.ElementDecl)
		return decl, ok && decl.IsReference
	})
}

// collectElementReferencesFromParticles collects element references from particles.
func collectElementReferencesFromParticles(particles []types.Particle) []*types.ElementDecl {
	visited := make(map[*types.ModelGroup]bool)
	return collectElementReferencesFromParticlesWithVisited(particles, visited)
}

// collectElementReferencesFromParticlesWithVisited collects element references with cycle detection.
func collectElementReferencesFromParticlesWithVisited(particles []types.Particle, visited map[*types.ModelGroup]bool) []*types.ElementDecl {
	return traversal.CollectFromParticlesWithVisited(particles, visited, func(p types.Particle) (*types.ElementDecl, bool) {
		decl, ok := p.(*types.ElementDecl)
		return decl, ok && decl.IsReference
	})
}
