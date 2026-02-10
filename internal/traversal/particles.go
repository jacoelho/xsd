package traversal

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/state"
)

// GetContentParticle extracts the particle from any content type.
func GetContentParticle(content model.Content) model.Particle {
	switch c := content.(type) {
	case *model.ElementContent:
		return c.Particle
	case *model.ComplexContent:
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
func WalkContentParticles(content model.Content, fn func(model.Particle) error) error {
	switch c := content.(type) {
	case *model.ElementContent:
		if c.Particle != nil {
			return fn(c.Particle)
		}
	case *model.ComplexContent:
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
func WalkParticles(particle model.Particle, fn func(model.Particle) error) error {
	if particle == nil {
		return nil
	}
	if err := fn(particle); err != nil {
		return err
	}
	if group, ok := particle.(*model.ModelGroup); ok {
		for _, child := range group.Particles {
			if err := WalkParticles(child, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// WalkParticlesWithVisited recursively visits particles and skips previously seen model groups.
func WalkParticlesWithVisited(particle model.Particle, visited map[*model.ModelGroup]bool, fn func(model.Particle) error) error {
	if particle == nil {
		return nil
	}
	if visited == nil {
		visited = make(map[*model.ModelGroup]bool)
	}
	if group, ok := particle.(*model.ModelGroup); ok {
		if visited[group] {
			return nil
		}
		visited[group] = true
	}
	if err := fn(particle); err != nil {
		return err
	}
	if group, ok := particle.(*model.ModelGroup); ok {
		for _, child := range group.Particles {
			if err := WalkParticlesWithVisited(child, visited, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectFromParticles[T any](particles []model.Particle, visited map[*model.ModelGroup]bool, collect func(model.Particle) (T, bool)) []T {
	if len(particles) == 0 {
		return nil
	}

	stack := state.NewStateStack[model.Particle](len(particles))
	for i := len(particles) - 1; i >= 0; i-- {
		if particles[i] != nil {
			stack.Push(particles[i])
		}
	}

	var result []T
	for stack.Len() > 0 {
		particle, _ := stack.Pop()

		group, ok := particle.(*model.ModelGroup)
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
func CollectFromContent[T any](content model.Content, collect func(model.Particle) (T, bool)) []T {
	switch c := content.(type) {
	case *model.ElementContent:
		return collectFromParticles([]model.Particle{c.Particle}, nil, collect)
	case *model.ComplexContent:
		var particles []model.Particle
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
func CollectFromParticlesWithVisited[T any](particles []model.Particle, visited map[*model.ModelGroup]bool, collect func(model.Particle) (T, bool)) []T {
	if len(particles) == 0 {
		return nil
	}
	if visited == nil {
		visited = make(map[*model.ModelGroup]bool)
	}
	return collectFromParticles(particles, visited, collect)
}
