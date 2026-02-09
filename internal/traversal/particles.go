package traversal

import (
	"github.com/jacoelho/xsd/internal/state"
	"github.com/jacoelho/xsd/internal/types"
)

// GetContentParticle extracts the particle from any content type.
func GetContentParticle(content types.Content) types.Particle {
	switch c := content.(type) {
	case *types.ElementContent:
		return c.Particle
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			return c.Extension.Particle
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			return c.Restriction.Particle
		}
	}
	return nil
}

// WalkContentParticles visits all particles in content.
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
	}
	return nil
}

// WalkParticles recursively visits all particles in a tree.
func WalkParticles(particle types.Particle, fn func(types.Particle) error) error {
	if particle == nil {
		return nil
	}
	if err := fn(particle); err != nil {
		return err
	}
	if group, ok := particle.(*types.ModelGroup); ok {
		for _, child := range group.Particles {
			if err := WalkParticles(child, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// WalkParticlesWithVisited recursively visits particles and skips previously seen model groups.
func WalkParticlesWithVisited(particle types.Particle, visited map[*types.ModelGroup]bool, fn func(types.Particle) error) error {
	if particle == nil {
		return nil
	}
	if visited == nil {
		visited = make(map[*types.ModelGroup]bool)
	}
	if group, ok := particle.(*types.ModelGroup); ok {
		if visited[group] {
			return nil
		}
		visited[group] = true
	}
	if err := fn(particle); err != nil {
		return err
	}
	if group, ok := particle.(*types.ModelGroup); ok {
		for _, child := range group.Particles {
			if err := WalkParticlesWithVisited(child, visited, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectFromParticles[T any](particles []types.Particle, visited map[*types.ModelGroup]bool, collect func(types.Particle) (T, bool)) []T {
	if len(particles) == 0 {
		return nil
	}

	stack := state.NewStateStack[types.Particle](len(particles))
	for i := len(particles) - 1; i >= 0; i-- {
		if particles[i] != nil {
			stack.Push(particles[i])
		}
	}

	var result []T
	for stack.Len() > 0 {
		particle, _ := stack.Pop()

		group, ok := particle.(*types.ModelGroup)
		if ok {
			if visited != nil {
				if visited[group] {
					continue
				}
				visited[group] = true
			}
			for i := len(group.Particles) - 1; i >= 0; i-- {
				if child := group.Particles[i]; child != nil {
					stack.Push(child)
				}
			}
		}

		if value, ok := collect(particle); ok {
			result = append(result, value)
		}
	}

	return result
}

// CollectFromContent collects values from all particles present in a content model.
func CollectFromContent[T any](content types.Content, collect func(types.Particle) (T, bool)) []T {
	switch c := content.(type) {
	case *types.ElementContent:
		return collectFromParticles([]types.Particle{c.Particle}, nil, collect)
	case *types.ComplexContent:
		var particles []types.Particle
		if c.Extension != nil && c.Extension.Particle != nil {
			particles = append(particles, c.Extension.Particle)
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			particles = append(particles, c.Restriction.Particle)
		}
		return collectFromParticles(particles, nil, collect)
	default:
		return nil
	}
}

// CollectFromParticlesWithVisited collects values and avoids revisiting model groups.
func CollectFromParticlesWithVisited[T any](particles []types.Particle, visited map[*types.ModelGroup]bool, collect func(types.Particle) (T, bool)) []T {
	if len(particles) == 0 {
		return nil
	}
	if visited == nil {
		visited = make(map[*types.ModelGroup]bool)
	}
	return collectFromParticles(particles, visited, collect)
}
