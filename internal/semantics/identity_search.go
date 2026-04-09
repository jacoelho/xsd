package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

type identitySearchMode uint8

const (
	identitySearchDirect identitySearchMode = iota
	identitySearchDescendant
)

type identitySearchState struct {
	visitedGroups map[*model.ModelGroup]bool
	visitedTypes  map[*model.ComplexType]bool
}

func newIdentitySearchState() *identitySearchState {
	return &identitySearchState{
		visitedGroups: make(map[*model.ModelGroup]bool),
		visitedTypes:  make(map[*model.ComplexType]bool),
	}
}

func resolveIdentityElementType(schema *parser.Schema, elementDecl *model.ElementDecl) (model.Type, error) {
	elementDecl = resolveElementReference(schema, elementDecl)
	if elementDecl == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}
	elementType := parser.ResolveTypeReference(schema, elementDecl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}
	return elementType, nil
}

func resolveIdentityElementComplexType(schema *parser.Schema, elementDecl *model.ElementDecl) (*model.ComplexType, error) {
	elementType, err := resolveIdentityElementType(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	ct, ok := elementType.(*model.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}
	return ct, nil
}

func searchIdentityParticles(schema *parser.Schema, particles []model.Particle, mode identitySearchMode, state *identitySearchState, visit func(*model.ElementDecl) bool) (found, unresolved bool) {
	for _, particle := range particles {
		found, particleUnresolved := searchIdentityParticle(schema, particle, mode, state, visit)
		if found {
			return true, false
		}
		unresolved = unresolved || particleUnresolved
	}
	return false, unresolved
}

func searchIdentityParticle(schema *parser.Schema, particle model.Particle, mode identitySearchMode, state *identitySearchState, visit func(*model.ElementDecl) bool) (found, unresolved bool) {
	if particle == nil || visit == nil {
		return false, false
	}
	if state == nil {
		state = newIdentitySearchState()
	}

	switch p := particle.(type) {
	case *model.ElementDecl:
		if visit(p) {
			return true, false
		}
		if mode != identitySearchDescendant {
			return false, false
		}
		return searchIdentityChildContent(schema, p, state, visit)

	case *model.ModelGroup:
		if p == nil || state.visitedGroups[p] {
			return false, false
		}
		state.visitedGroups[p] = true
		return searchIdentityParticles(schema, p.Particles, mode, state, visit)

	case *model.AnyElement:
		return false, true
	}

	return false, false
}

func searchIdentityChildContent(schema *parser.Schema, decl *model.ElementDecl, state *identitySearchState, visit func(*model.ElementDecl) bool) (found, unresolved bool) {
	ct, err := resolveIdentityElementComplexType(schema, decl)
	if err != nil || ct == nil {
		return false, false
	}
	if state.visitedTypes[ct] {
		return false, false
	}
	state.visitedTypes[ct] = true
	return searchIdentityDescendantContent(schema, ct.Content(), state, visit)
}

func searchIdentityDescendantContent(schema *parser.Schema, content model.Content, state *identitySearchState, visit func(*model.ElementDecl) bool) (found, unresolved bool) {
	switch c := content.(type) {
	case *model.ElementContent:
		if c.Particle != nil {
			found, _ = searchIdentityParticle(schema, c.Particle, identitySearchDescendant, state, visit)
			return found, false
		}
	case *model.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			found, _ = searchIdentityParticle(schema, c.Extension.Particle, identitySearchDescendant, state, visit)
			if found {
				return true, false
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			found, _ = searchIdentityParticle(schema, c.Restriction.Particle, identitySearchDescendant, state, visit)
			return found, false
		}
	}
	return false, false
}
