package semanticresolve

import "github.com/jacoelho/xsd/internal/model"

func collectConstraintElementsFromContent(content model.Content) []*model.ElementDecl {
	state := newIdentityTraversalState()
	out := make([]*model.ElementDecl, 0)
	walkIdentityContent(content, state, func(elem *model.ElementDecl) {
		if elem == nil || elem.IsReference || len(elem.Constraints) == 0 {
			return
		}
		out = append(out, elem)
	})
	return out
}
