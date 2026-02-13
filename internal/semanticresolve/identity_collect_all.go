package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func collectAllIdentityConstraintsWithIndex(sch *parser.Schema, index *iterationIndex) []*types.IdentityConstraint {
	var all []*types.IdentityConstraint
	state := newIdentityTraversalState()
	collectConstraints := func(elem *types.ElementDecl) {
		if elem == nil || len(elem.Constraints) == 0 {
			return
		}
		all = append(all, elem.Constraints...)
	}

	collectFromContent := func(content types.Content) {
		walkIdentityContent(content, state, collectConstraints)
	}

	for _, qname := range index.elementQNames {
		decl := sch.ElementDecls[qname]
		all = append(all, decl.Constraints...)
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range index.typeQNames {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range index.groupQNames {
		group := sch.Groups[qname]
		walkIdentityParticles(group.Particles, state, collectConstraints)
	}

	return all
}
