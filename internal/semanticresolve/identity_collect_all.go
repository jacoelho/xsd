package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// collectAllIdentityConstraints collects all identity constraints from the schema
// including constraints on local elements in content models.
func collectAllIdentityConstraints(sch *parser.Schema) []*types.IdentityConstraint {
	var all []*types.IdentityConstraint
	visitedGroups := make(map[*types.ModelGroup]bool)
	visitedTypes := make(map[*types.ComplexType]bool)

	collectFromContent := func(content types.Content) {
		all = append(all, collectIdentityConstraintsFromContentWithVisited(content, visitedGroups, visitedTypes)...)
	}

	for _, qname := range sortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		all = append(all, decl.Constraints...)
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range sortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range sortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		all = append(all, collectIdentityConstraintsFromParticlesWithVisited(group.Particles, visitedGroups, visitedTypes)...)
	}

	return all
}
