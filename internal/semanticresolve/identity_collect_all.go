package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

// collectAllIdentityConstraints collects all identity constraints from the schema
// including constraints on local elements in content models.
func collectAllIdentityConstraints(sch *parser.Schema) []*types.IdentityConstraint {
	var all []*types.IdentityConstraint
	visitedGroups := make(map[*types.ModelGroup]bool)
	visitedTypes := make(map[*types.ComplexType]bool)
	var collectParticles func([]types.Particle, map[*types.ModelGroup]bool, map[*types.ComplexType]bool) []*types.IdentityConstraint
	collectParticles = func(particles []types.Particle, visited map[*types.ModelGroup]bool, visitedComplex map[*types.ComplexType]bool) []*types.IdentityConstraint {
		return collectFromParticlesWithVisited(particles, visited, visitedComplex, func(p *types.ElementDecl, visitedNestedGroups map[*types.ModelGroup]bool, visitedNestedTypes map[*types.ComplexType]bool) []*types.IdentityConstraint {
			if p == nil {
				return nil
			}
			constraints := append([]*types.IdentityConstraint(nil), p.Constraints...)
			ct, ok := p.Type.(*types.ComplexType)
			if !ok || ct == nil {
				return constraints
			}
			if visitedNestedTypes[ct] {
				return constraints
			}
			visitedNestedTypes[ct] = true
			constraints = append(constraints, collectFromContentParticlesWithVisited(ct.Content(), visitedNestedGroups, visitedNestedTypes, collectParticles)...)
			return constraints
		})
	}

	collectFromContent := func(content types.Content) {
		all = append(all, collectFromContentParticlesWithVisited(content, visitedGroups, visitedTypes, collectParticles)...)
	}

	for _, qname := range traversal.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		all = append(all, decl.Constraints...)
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range traversal.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range traversal.SortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		all = append(all, collectParticles(group.Particles, visitedGroups, visitedTypes)...)
	}

	return all
}
