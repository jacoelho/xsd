package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

func resolvePathElementDecl(schema *parser.Schema, startDecl *model.ElementDecl, steps []runtime.Step) (*model.ElementDecl, error) {
	current := resolveElementReference(schema, startDecl)
	descendantNext := false

	for _, step := range steps {
		switch step.Axis {
		case runtime.AxisDescendantOrSelf:
			if !step.Test.Any {
				return nil, fmt.Errorf("xpath uses disallowed axis")
			}
			if descendantNext {
				return nil, fmt.Errorf("xpath step is missing a node test")
			}
			descendantNext = true
			continue

		case runtime.AxisSelf:
			if descendantNext {
				if step.Test.Any {
					return nil, fmt.Errorf("%w: descendant self step", ErrXPathUnresolvable)
				}
				return nil, fmt.Errorf("xpath step is missing a node test")
			}
			if !step.Test.Any && current != nil && !nodeTestMatchesQName(step.Test, current.Name) {
				return nil, fmt.Errorf("xpath self step does not match current element")
			}
			continue

		case runtime.AxisChild:
		default:
			return nil, fmt.Errorf("xpath uses disallowed axis")
		}

		if isWildcardNodeTest(step.Test) {
			return nil, fmt.Errorf("%w: wildcard node test", ErrXPathUnresolvable)
		}

		var err error
		if descendantNext {
			current, err = findElementDeclDescendant(schema, current, step.Test)
			descendantNext = false
		} else {
			current, err = findElementDecl(schema, current, step.Test)
		}
		if err != nil {
			return nil, err
		}
		current = resolveElementReference(schema, current)
	}

	if descendantNext {
		return nil, fmt.Errorf("xpath step is missing a node test")
	}
	if current == nil {
		return nil, fmt.Errorf("cannot resolve element declaration")
	}
	return current, nil
}

func findElementDeclDescendant(schema *parser.Schema, elementDecl *model.ElementDecl, test runtime.NodeTest) (*model.ElementDecl, error) {
	ct, err := resolveIdentityElementComplexType(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	state := newIdentitySearchState()
	state.visitedTypes[ct] = true
	decl, err := findElementDeclInContentWithMode(schema, ct.Content(), test, identitySearchDescendant, state)
	if err != nil && ct.Abstract {
		return nil, fmt.Errorf("%w: %w", ErrXPathUnresolvable, err)
	}
	return decl, err
}

func findElementDeclInContentDescendant(schema *parser.Schema, content model.Content, test runtime.NodeTest, visited map[*model.ComplexType]struct{}) (*model.ElementDecl, error) {
	state := newIdentitySearchState()
	for ct := range visited {
		state.visitedTypes[ct] = true
	}
	return findElementDeclInContentWithMode(schema, content, test, identitySearchDescendant, state)
}

func findElementDecl(schema *parser.Schema, elementDecl *model.ElementDecl, test runtime.NodeTest) (*model.ElementDecl, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}
	ct, err := resolveIdentityElementComplexType(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	return findElementDeclInContentWithMode(schema, ct.Content(), test, identitySearchDirect, newIdentitySearchState())
}

func findElementDeclInContent(content model.Content, test runtime.NodeTest) (*model.ElementDecl, error) {
	return findElementDeclInContentWithMode(nil, content, test, identitySearchDirect, newIdentitySearchState())
}

func findElementDeclInContentWithMode(schema *parser.Schema, content model.Content, test runtime.NodeTest, mode identitySearchMode, state *identitySearchState) (*model.ElementDecl, error) {
	var result *model.ElementDecl
	visit := func(elem *model.ElementDecl) bool {
		candidate := elem
		if mode == identitySearchDescendant {
			candidate = resolveElementReference(schema, elem)
		}
		if candidate == nil || !nodeTestMatchesQName(test, candidate.Name) {
			return false
		}
		result = candidate
		return true
	}
	searchParticle := func(particle model.Particle) (bool, bool) {
		if particle == nil {
			return false, false
		}
		return searchIdentityParticle(schema, particle, mode, state, visit)
	}
	notFound := func(scope string) error {
		return fmt.Errorf("element '%s' not found in %s", formatNodeTest(test), scope)
	}
	wildcardErr := func() error {
		return fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}

	switch c := content.(type) {
	case *model.ElementContent:
		found, unresolved := searchParticle(c.Particle)
		if found {
			return result, nil
		}
		if unresolved {
			return nil, wildcardErr()
		}
		return nil, notFound("content model")

	case *model.SimpleContent:
		return nil, notFound("simple content")

	case *model.ComplexContent:
		switch mode {
		case identitySearchDescendant:
			if c.Extension != nil && c.Extension.Particle != nil {
				if found, _ := searchParticle(c.Extension.Particle); found {
					return result, nil
				}
			}
			if c.Restriction != nil && c.Restriction.Particle != nil {
				found, unresolved := searchParticle(c.Restriction.Particle)
				if found {
					return result, nil
				}
				if unresolved {
					return nil, wildcardErr()
				}
			}

		default:
			unresolved := false
			if c.Extension != nil && c.Extension.Particle != nil {
				found, childUnresolved := searchParticle(c.Extension.Particle)
				if found {
					return result, nil
				}
				unresolved = unresolved || childUnresolved
			}
			if c.Restriction != nil && c.Restriction.Particle != nil {
				found, childUnresolved := searchParticle(c.Restriction.Particle)
				if found {
					return result, nil
				}
				unresolved = unresolved || childUnresolved
			}
			if unresolved {
				return nil, wildcardErr()
			}
		}
		return nil, notFound("content model")

	case *model.EmptyContent:
		return nil, notFound("empty content")
	}

	return nil, notFound("content model")
}
