package semanticresolve

import "github.com/jacoelho/xsd/internal/types"

func collectFromContentParticlesWithVisited[T any](content types.Content, visited map[*types.ModelGroup]bool, visitedTypes map[*types.ComplexType]bool, collect func([]types.Particle, map[*types.ModelGroup]bool, map[*types.ComplexType]bool) []T) []T {
	if content == nil {
		return nil
	}
	if visited == nil {
		visited = make(map[*types.ModelGroup]bool)
	}
	if visitedTypes == nil {
		visitedTypes = make(map[*types.ComplexType]bool)
	}
	var particles []types.Particle
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			particles = append(particles, c.Particle)
		}
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			particles = append(particles, c.Extension.Particle)
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			particles = append(particles, c.Restriction.Particle)
		}
	}
	var out []T
	for _, particle := range particles {
		out = append(out, collect([]types.Particle{particle}, visited, visitedTypes)...)
	}
	return out
}

func collectFromParticlesWithVisited[T any](particles []types.Particle, visited map[*types.ModelGroup]bool, visitedTypes map[*types.ComplexType]bool, collectElement func(*types.ElementDecl, map[*types.ModelGroup]bool, map[*types.ComplexType]bool) []T) []T {
	if len(particles) == 0 {
		return nil
	}
	if visited == nil {
		visited = make(map[*types.ModelGroup]bool)
	}
	if visitedTypes == nil {
		visitedTypes = make(map[*types.ComplexType]bool)
	}
	var out []T
	for _, particle := range particles {
		switch p := particle.(type) {
		case *types.ElementDecl:
			out = append(out, collectElement(p, visited, visitedTypes)...)
		case *types.ModelGroup:
			if p == nil || visited[p] {
				continue
			}
			visited[p] = true
			out = append(out, collectFromParticlesWithVisited(p.Particles, visited, visitedTypes, collectElement)...)
		}
	}
	return out
}
