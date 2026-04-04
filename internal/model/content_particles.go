package model

import "github.com/jacoelho/xsd/internal/stack"

// GetContentParticle extracts the particle from any content type.
func GetContentParticle(content Content) Particle {
	switch c := content.(type) {
	case *ElementContent:
		return c.Particle
	case *ComplexContent:
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
func WalkContentParticles(content Content, fn func(Particle) error) error {
	switch c := content.(type) {
	case *ElementContent:
		if c.Particle != nil {
			return fn(c.Particle)
		}
	case *ComplexContent:
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

func collectFromParticles[T any](particles []Particle, visited map[*ModelGroup]bool, collect func(Particle) (T, bool)) []T {
	if len(particles) == 0 {
		return nil
	}

	particleStack := stack.NewStack[Particle](len(particles))
	for i := len(particles) - 1; i >= 0; i-- {
		if particles[i] != nil {
			particleStack.Push(particles[i])
		}
	}

	var result []T
	for particleStack.Len() > 0 {
		particle, _ := particleStack.Pop()

		group, ok := particle.(*ModelGroup)
		if ok {
			if visited != nil {
				if visited[group] {
					continue
				}
				visited[group] = true
			}
			for i := len(group.Particles) - 1; i >= 0; i-- {
				if child := group.Particles[i]; child != nil {
					particleStack.Push(child)
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
func CollectFromContent[T any](content Content, collect func(Particle) (T, bool)) []T {
	visited := make(map[*ModelGroup]bool)
	switch c := content.(type) {
	case *ElementContent:
		return collectFromParticles([]Particle{c.Particle}, visited, collect)
	case *ComplexContent:
		var particles []Particle
		if c.Extension != nil && c.Extension.Particle != nil {
			particles = append(particles, c.Extension.Particle)
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			particles = append(particles, c.Restriction.Particle)
		}
		return collectFromParticles(particles, visited, collect)
	default:
		return nil
	}
}

// CollectFromParticlesWithVisited collects values and avoids revisiting model groups.
func CollectFromParticlesWithVisited[T any](particles []Particle, visited map[*ModelGroup]bool, collect func(Particle) (T, bool)) []T {
	if len(particles) == 0 {
		return nil
	}
	if visited == nil {
		visited = make(map[*ModelGroup]bool)
	}
	return collectFromParticles(particles, visited, collect)
}
