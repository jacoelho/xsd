package semanticresolve

import "github.com/jacoelho/xsd/internal/model"

func collectFromContentParticlesWithVisited[T any](content model.Content, visited map[*model.ModelGroup]bool, visitedTypes map[*model.ComplexType]bool, collect func([]model.Particle, map[*model.ModelGroup]bool, map[*model.ComplexType]bool) []T) []T {
	if content == nil {
		return nil
	}
	if visited == nil {
		visited = make(map[*model.ModelGroup]bool)
	}
	if visitedTypes == nil {
		visitedTypes = make(map[*model.ComplexType]bool)
	}
	var particles []model.Particle
	switch c := content.(type) {
	case *model.ElementContent:
		if c.Particle != nil {
			particles = append(particles, c.Particle)
		}
	case *model.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			particles = append(particles, c.Extension.Particle)
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			particles = append(particles, c.Restriction.Particle)
		}
	}
	var out []T
	for _, particle := range particles {
		out = append(out, collect([]model.Particle{particle}, visited, visitedTypes)...)
	}
	return out
}

func collectFromParticlesWithVisited[T any](particles []model.Particle, visited map[*model.ModelGroup]bool, visitedTypes map[*model.ComplexType]bool, collectElement func(*model.ElementDecl, map[*model.ModelGroup]bool, map[*model.ComplexType]bool) []T) []T {
	if len(particles) == 0 {
		return nil
	}
	if visited == nil {
		visited = make(map[*model.ModelGroup]bool)
	}
	if visitedTypes == nil {
		visitedTypes = make(map[*model.ComplexType]bool)
	}
	var out []T
	for _, particle := range particles {
		switch p := particle.(type) {
		case *model.ElementDecl:
			out = append(out, collectElement(p, visited, visitedTypes)...)
		case *model.ModelGroup:
			if p == nil || visited[p] {
				continue
			}
			visited[p] = true
			out = append(out, collectFromParticlesWithVisited(p.Particles, visited, visitedTypes, collectElement)...)
		}
	}
	return out
}
