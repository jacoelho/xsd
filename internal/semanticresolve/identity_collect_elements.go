package semanticresolve

import "github.com/jacoelho/xsd/internal/types"

func collectConstraintElementsFromContent(content types.Content) []*types.ElementDecl {
	visitedGroups := make(map[*types.ModelGroup]bool)
	visitedTypes := make(map[*types.ComplexType]bool)
	var collectParticles func([]types.Particle, map[*types.ModelGroup]bool, map[*types.ComplexType]bool) []*types.ElementDecl
	collectParticles = func(particles []types.Particle, visited map[*types.ModelGroup]bool, visitedComplex map[*types.ComplexType]bool) []*types.ElementDecl {
		return collectFromParticlesWithVisited(particles, visited, visitedComplex, func(p *types.ElementDecl, visitedNestedGroups map[*types.ModelGroup]bool, visitedNestedTypes map[*types.ComplexType]bool) []*types.ElementDecl {
			var elements []*types.ElementDecl
			if p != nil && !p.IsReference && len(p.Constraints) > 0 {
				elements = append(elements, p)
			}
			if p == nil {
				return elements
			}
			ct, ok := p.Type.(*types.ComplexType)
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
