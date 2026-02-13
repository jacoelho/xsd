package semanticresolve

import "github.com/jacoelho/xsd/internal/types"

func collectConstraintElementsFromContent(content types.Content) []*types.ElementDecl {
	state := newIdentityTraversalState()
	out := make([]*types.ElementDecl, 0)
	walkIdentityContent(content, state, func(elem *types.ElementDecl) {
		if elem == nil || elem.IsReference || len(elem.Constraints) == 0 {
			return
		}
		out = append(out, elem)
	})
	return out
}
