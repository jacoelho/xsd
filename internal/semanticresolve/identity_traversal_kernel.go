package semanticresolve

import model "github.com/jacoelho/xsd/internal/types"

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
