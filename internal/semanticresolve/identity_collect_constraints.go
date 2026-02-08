package semanticresolve

import "github.com/jacoelho/xsd/internal/types"

// collectIdentityConstraintsFromContentWithVisited collects identity constraints with cycle detection.
func collectIdentityConstraintsFromContentWithVisited(content types.Content, visited map[*types.ModelGroup]bool, visitedTypes map[*types.ComplexType]bool) []*types.IdentityConstraint {
	return collectFromContentParticlesWithVisited(content, visited, visitedTypes, collectIdentityConstraintsFromParticlesWithVisited)
}

// collectIdentityConstraintsFromParticlesWithVisited collects identity constraints with cycle detection.
func collectIdentityConstraintsFromParticlesWithVisited(particles []types.Particle, visited map[*types.ModelGroup]bool, visitedTypes map[*types.ComplexType]bool) []*types.IdentityConstraint {
	return collectFromParticlesWithVisited(particles, visited, visitedTypes, func(p *types.ElementDecl, visitedGroups map[*types.ModelGroup]bool, visitedComplex map[*types.ComplexType]bool) []*types.IdentityConstraint {
		if p == nil {
			return nil
		}
		constraints := append([]*types.IdentityConstraint(nil), p.Constraints...)
		ct, ok := p.Type.(*types.ComplexType)
		if !ok || ct == nil {
			return constraints
		}
		if visitedComplex[ct] {
			return constraints
		}
		visitedComplex[ct] = true
		constraints = append(constraints, collectIdentityConstraintsFromContentWithVisited(ct.Content(), visitedGroups, visitedComplex)...)
		return constraints
	})
}
