package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
)

// collectAllIdentityConstraints collects all identity constraints from the schema
// including constraints on local elements in content models.
func collectAllIdentityConstraints(sch *parser.Schema) []*model.IdentityConstraint {
	var all []*model.IdentityConstraint
	visitedGroups := make(map[*model.ModelGroup]bool)
	visitedTypes := make(map[*model.ComplexType]bool)
	var collectParticles func([]model.Particle, map[*model.ModelGroup]bool, map[*model.ComplexType]bool) []*model.IdentityConstraint
	collectParticles = func(particles []model.Particle, visited map[*model.ModelGroup]bool, visitedComplex map[*model.ComplexType]bool) []*model.IdentityConstraint {
		return collectFromParticlesWithVisited(particles, visited, visitedComplex, func(p *model.ElementDecl, visitedNestedGroups map[*model.ModelGroup]bool, visitedNestedTypes map[*model.ComplexType]bool) []*model.IdentityConstraint {
			if p == nil {
				return nil
			}
			constraints := append([]*model.IdentityConstraint(nil), p.Constraints...)
			ct, ok := p.Type.(*model.ComplexType)
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

	collectFromContent := func(content model.Content) {
		all = append(all, collectFromContentParticlesWithVisited(content, visitedGroups, visitedTypes, collectParticles)...)
	}

	for _, qname := range traversal.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		all = append(all, decl.Constraints...)
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range traversal.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range traversal.SortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		all = append(all, collectParticles(group.Particles, visitedGroups, visitedTypes)...)
	}

	return all
}
