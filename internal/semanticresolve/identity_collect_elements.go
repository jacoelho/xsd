package semanticresolve

import "github.com/jacoelho/xsd/internal/types"

func collectConstraintElementsFromContent(content types.Content) []*types.ElementDecl {
	visited := make(map[*types.ModelGroup]bool)
	visitedTypes := make(map[*types.ComplexType]bool)
	return collectConstraintElementsFromContentWithVisited(content, visited, visitedTypes)
}

func collectConstraintElementsFromContentWithVisited(content types.Content, visited map[*types.ModelGroup]bool, visitedTypes map[*types.ComplexType]bool) []*types.ElementDecl {
	return collectFromContentParticlesWithVisited(content, visited, visitedTypes, collectConstraintElementsFromParticlesWithVisited)
}

func collectConstraintElementsFromParticlesWithVisited(particles []types.Particle, visited map[*types.ModelGroup]bool, visitedTypes map[*types.ComplexType]bool) []*types.ElementDecl {
	return collectFromParticlesWithVisited(particles, visited, visitedTypes, func(p *types.ElementDecl, visitedGroups map[*types.ModelGroup]bool, visitedComplex map[*types.ComplexType]bool) []*types.ElementDecl {
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
		if visitedComplex[ct] {
			return elements
		}
		visitedComplex[ct] = true
		elements = append(elements, collectConstraintElementsFromContentWithVisited(ct.Content(), visitedGroups, visitedComplex)...)
		return elements
	})
}
