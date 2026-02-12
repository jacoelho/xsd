package semanticresolve

import (
	parser "github.com/jacoelho/xsd/internal/parser"
	model "github.com/jacoelho/xsd/internal/types"
)

func collectAllIdentityConstraintsWithIndex(sch *parser.Schema, index *iterationIndex) []*model.IdentityConstraint {
	var all []*model.IdentityConstraint
	state := newIdentityTraversalState()
	collectConstraints := func(elem *model.ElementDecl) {
		if elem == nil || len(elem.Constraints) == 0 {
			return
		}
		all = append(all, elem.Constraints...)
	}

	collectFromContent := func(content model.Content) {
		walkIdentityContent(content, state, collectConstraints)
	}

	for _, qname := range index.elementQNames {
		decl := sch.ElementDecls[qname]
		all = append(all, decl.Constraints...)
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range index.typeQNames {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, qname := range index.groupQNames {
		group := sch.Groups[qname]
		walkIdentityParticles(group.Particles, state, collectConstraints)
	}

	return all
}
