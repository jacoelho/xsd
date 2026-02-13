package semanticresolve

import "github.com/jacoelho/xsd/internal/types"

type identityTraversalState struct {
	visitedGroups map[*types.ModelGroup]bool
	visitedTypes  map[*types.ComplexType]bool
}

func newIdentityTraversalState() *identityTraversalState {
	return &identityTraversalState{
		visitedGroups: make(map[*types.ModelGroup]bool),
		visitedTypes:  make(map[*types.ComplexType]bool),
	}
}

func walkIdentityContent(content types.Content, state *identityTraversalState, visit func(*types.ElementDecl)) {
	if content == nil {
		return
	}
	if state == nil {
		state = newIdentityTraversalState()
	}

	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			walkIdentityParticle(c.Particle, state, visit)
		}
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			walkIdentityParticle(c.Extension.Particle, state, visit)
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			walkIdentityParticle(c.Restriction.Particle, state, visit)
		}
	}
}

func walkIdentityParticles(particles []types.Particle, state *identityTraversalState, visit func(*types.ElementDecl)) {
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

func walkIdentityParticle(particle types.Particle, state *identityTraversalState, visit func(*types.ElementDecl)) {
	if particle == nil || state == nil || visit == nil {
		return
	}

	switch p := particle.(type) {
	case *types.ElementDecl:
		visit(p)
		if p == nil {
			return
		}
		ct, ok := p.Type.(*types.ComplexType)
		if !ok || ct == nil {
			return
		}
		if state.visitedTypes[ct] {
			return
		}
		state.visitedTypes[ct] = true
		walkIdentityContent(ct.Content(), state, visit)
	case *types.ModelGroup:
		if p == nil || state.visitedGroups[p] {
			return
		}
		state.visitedGroups[p] = true
		for _, child := range p.Particles {
			walkIdentityParticle(child, state, visit)
		}
	}
}
