package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

type identityTraversalState struct {
	visitedGroups map[*model.ModelGroup]bool
	visitedTypes  map[*model.ComplexType]bool
}

func newIdentityTraversalState() *identityTraversalState {
	return &identityTraversalState{
		visitedGroups: make(map[*model.ModelGroup]bool),
		visitedTypes:  make(map[*model.ComplexType]bool),
	}
}

func walkIdentityContent(content model.Content, state *identityTraversalState, visit func(*model.ElementDecl)) {
	if content == nil {
		return
	}
	if state == nil {
		state = newIdentityTraversalState()
	}
	switch c := content.(type) {
	case *model.ElementContent:
		if c.Particle != nil {
			walkIdentityParticle(c.Particle, state, visit)
		}
	case *model.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			walkIdentityParticle(c.Extension.Particle, state, visit)
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			walkIdentityParticle(c.Restriction.Particle, state, visit)
		}
	}
}

func walkIdentityParticles(particles []model.Particle, state *identityTraversalState, visit func(*model.ElementDecl)) {
	if len(particles) == 0 {
		return
	}
	if state == nil {
		state = newIdentityTraversalState()
	}
	for _, particle := range particles {
		walkIdentityParticle(particle, state, visit)
	}
}

func walkIdentityParticle(particle model.Particle, state *identityTraversalState, visit func(*model.ElementDecl)) {
	if particle == nil || state == nil || visit == nil {
		return
	}
	switch p := particle.(type) {
	case *model.ElementDecl:
		visit(p)
		if p == nil {
			return
		}
		ct, ok := p.Type.(*model.ComplexType)
		if !ok || ct == nil {
			return
		}
		if state.visitedTypes[ct] {
			return
		}
		state.visitedTypes[ct] = true
		walkIdentityContent(ct.Content(), state, visit)
	case *model.ModelGroup:
		if p == nil || state.visitedGroups[p] {
			return
		}
		state.visitedGroups[p] = true
		for _, child := range p.Particles {
			walkIdentityParticle(child, state, visit)
		}
	}
}

// CollectConstraintElementsFromContent returns non-reference elements with
// identity constraints found in the given content tree.
func CollectConstraintElementsFromContent(content model.Content) []*model.ElementDecl {
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

// CollectAllIdentityConstraints returns all identity constraints in deterministic
// schema traversal order.
func CollectAllIdentityConstraints(sch *parser.Schema) []*model.IdentityConstraint {
	if sch == nil {
		return nil
	}
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

	for _, name := range model.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[name]
		all = append(all, decl.Constraints...)
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}
	for _, name := range model.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[name]
		if ct, ok := typ.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}
	for _, name := range model.SortedMapKeys(sch.Groups) {
		group := sch.Groups[name]
		walkIdentityParticles(group.Particles, state, collectConstraints)
	}
	return all
}

// CollectLocalConstraintElements returns local elements with identity
// constraints in deterministic order.
func CollectLocalConstraintElements(sch *parser.Schema) []*model.ElementDecl {
	if sch == nil {
		return nil
	}
	seen := make(map[*model.ElementDecl]bool)
	out := make([]*model.ElementDecl, 0)
	collect := func(content model.Content) {
		for _, elem := range CollectConstraintElementsFromContent(content) {
			if elem == nil || elem.IsReference || len(elem.Constraints) == 0 {
				continue
			}
			if seen[elem] {
				continue
			}
			seen[elem] = true
			out = append(out, elem)
		}
	}
	for _, name := range model.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[name]
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			collect(ct.Content())
		}
	}
	for _, name := range model.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[name]
		if ct, ok := typ.(*model.ComplexType); ok {
			collect(ct.Content())
		}
	}
	return out
}
