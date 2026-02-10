package semanticresolve

import "github.com/jacoelho/xsd/internal/model"

func collectConstraintElementsFromContent(content model.Content) []*model.ElementDecl {
	visitedGroups := make(map[*model.ModelGroup]bool)
	visitedTypes := make(map[*model.ComplexType]bool)
	var collectParticles func([]model.Particle, map[*model.ModelGroup]bool, map[*model.ComplexType]bool) []*model.ElementDecl
	collectParticles = func(particles []model.Particle, visited map[*model.ModelGroup]bool, visitedComplex map[*model.ComplexType]bool) []*model.ElementDecl {
		return collectFromParticlesWithVisited(particles, visited, visitedComplex, func(p *model.ElementDecl, visitedNestedGroups map[*model.ModelGroup]bool, visitedNestedTypes map[*model.ComplexType]bool) []*model.ElementDecl {
			var elements []*model.ElementDecl
			if p != nil && !p.IsReference && len(p.Constraints) > 0 {
				elements = append(elements, p)
			}
			if p == nil {
				return elements
			}
			ct, ok := p.Type.(*model.ComplexType)
			if !ok || ct == nil {
				return elements
			}
			if visitedNestedTypes[ct] {
				return elements
			}
			visitedNestedTypes[ct] = true
			elements = append(elements, collectFromContentParticlesWithVisited(ct.Content(), visitedNestedGroups, visitedNestedTypes, collectParticles)...)
			return elements
		})
	}

	return collectFromContentParticlesWithVisited(content, visitedGroups, visitedTypes, collectParticles)
}
