package schemacheck

import (
	"github.com/jacoelho/xsd/internal/types"
)

// GetContentParticle extracts the particle from any content type.
// Returns nil if content doesn't contain a particle.
func GetContentParticle(content types.Content) types.Particle {
	switch c := content.(type) {
	case *types.ElementContent:
		return c.Particle
	case *types.SimpleContent:
		return nil
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			return c.Extension.Particle
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			return c.Restriction.Particle
		}
		return nil
	case *types.EmptyContent:
		return nil
	default:
		return nil
	}
}

// WalkContentParticles visits all particles in content (extension + restriction).
// Calls fn for each particle found. Returns the first error encountered.
func WalkContentParticles(content types.Content, fn func(types.Particle) error) error {
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			return fn(c.Particle)
		}
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			if err := fn(c.Extension.Particle); err != nil {
				return err
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			if err := fn(c.Restriction.Particle); err != nil {
				return err
			}
		}
	case *types.SimpleContent, *types.EmptyContent:
	}
	return nil
}

// WalkParticles recursively visits all particles in a tree.
// Calls fn for each particle found, including nested particles in ModelGroups.
// Returns the first error encountered.
func WalkParticles(particle types.Particle, fn func(types.Particle) error) error {
	if particle == nil {
		return nil
	}

	if err := fn(particle); err != nil {
		return err
	}

	switch p := particle.(type) {
	case *types.ModelGroup:
		for _, child := range p.Particles {
			if err := WalkParticles(child, fn); err != nil {
				return err
			}
		}
	case *types.ElementDecl, *types.AnyElement, *types.GroupRef:
	}
	return nil
}

// CollectElements returns all ElementDecl in a particle tree.
func CollectElements(particle types.Particle) []*types.ElementDecl {
	var result []*types.ElementDecl
	if err := WalkParticles(particle, func(p types.Particle) error {
		if elem, ok := p.(*types.ElementDecl); ok {
			result = append(result, elem)
		}
		return nil
	}); err != nil {
		return nil
	}
	return result
}

// CollectWildcards returns all AnyElement in a particle tree.
func CollectWildcards(particle types.Particle) []*types.AnyElement {
	var result []*types.AnyElement
	if err := WalkParticles(particle, func(p types.Particle) error {
		if wildcard, ok := p.(*types.AnyElement); ok {
			result = append(result, wildcard)
		}
		return nil
	}); err != nil {
		return nil
	}
	return result
}
