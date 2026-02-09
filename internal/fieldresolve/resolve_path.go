package fieldresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

func resolvePathElementDecl(schema *parser.Schema, startDecl *types.ElementDecl, steps []xpath.Step) (*types.ElementDecl, error) {
	current := resolveElementReference(schema, startDecl)
	descendantNext := false

	for _, step := range steps {
		switch step.Axis {
		case xpath.AxisDescendantOrSelf:
			if !step.Test.Any {
				return nil, fmt.Errorf("xpath uses disallowed axis")
			}
			if descendantNext {
				return nil, fmt.Errorf("xpath step is missing a node test")
			}
			descendantNext = true
			continue
		case xpath.AxisSelf:
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
		case xpath.AxisChild:
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

// findElementDeclDescendant searches for an element declaration at any depth in the content model.
func findElementDeclDescendant(schema *parser.Schema, elementDecl *types.ElementDecl, test xpath.NodeTest) (*types.ElementDecl, error) {
	elementDecl = resolveElementReference(schema, elementDecl)
	elementType := typeops.ResolveTypeReference(schema, elementDecl.Type, typeops.TypeReferenceMustExist)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	ct, ok := elementType.(*types.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}

	visited := map[*types.ComplexType]struct{}{
		ct: {},
	}
	decl, err := findElementDeclInContentDescendant(schema, ct.Content(), test, visited)
	if err != nil && ct.Abstract {
		return nil, fmt.Errorf("%w: %w", ErrXPathUnresolvable, err)
	}
	return decl, err
}

// findElementDeclInContentDescendant searches for an element declaration at any depth in content.
func findElementDeclInContentDescendant(schema *parser.Schema, content types.Content, test xpath.NodeTest, visited map[*types.ComplexType]struct{}) (*types.ElementDecl, error) {
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			return findElementDeclInParticleDescendant(schema, c.Particle, test, visited)
		}
	case *types.SimpleContent:
		return nil, fmt.Errorf("element '%s' not found in simple content", formatNodeTest(test))
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			if decl, err := findElementDeclInParticleDescendant(schema, c.Extension.Particle, test, visited); err == nil {
				return decl, nil
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			return findElementDeclInParticleDescendant(schema, c.Restriction.Particle, test, visited)
		}
	case *types.EmptyContent:
		return nil, fmt.Errorf("element '%s' not found in empty content", formatNodeTest(test))
	}

	return nil, fmt.Errorf("element '%s' not found in content model", formatNodeTest(test))
}

// findElementDeclInParticleDescendant searches for an element declaration at any depth in a particle tree.
func findElementDeclInParticleDescendant(schema *parser.Schema, particle types.Particle, test xpath.NodeTest, visited map[*types.ComplexType]struct{}) (*types.ElementDecl, error) {
	switch p := particle.(type) {
	case *types.ElementDecl:
		elem := resolveElementReference(schema, p)
		if nodeTestMatchesQName(test, elem.Name) {
			return elem, nil
		}
		if elem.Type != nil {
			if resolvedType := typeops.ResolveTypeReference(schema, elem.Type, typeops.TypeReferenceMustExist); resolvedType != nil {
				if ct, ok := resolvedType.(*types.ComplexType); ok {
					if _, seen := visited[ct]; !seen {
						visited[ct] = struct{}{}
						if decl, err := findElementDeclInContentDescendant(schema, ct.Content(), test, visited); err == nil {
							return decl, nil
						}
					}
				}
			}
		}
	case *types.ModelGroup:
		var unresolvedErr error
		for _, childParticle := range p.Particles {
			if decl, err := findElementDeclInParticleDescendant(schema, childParticle, test, visited); err == nil {
				return decl, nil
			} else if errors.Is(err, ErrXPathUnresolvable) && unresolvedErr == nil {
				unresolvedErr = err
			}
		}
		if unresolvedErr != nil {
			return nil, unresolvedErr
		}
		return nil, fmt.Errorf("element '%s' not found in model group", formatNodeTest(test))
	case *types.AnyElement:
		return nil, fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}

	return nil, fmt.Errorf("element '%s' not found in particle", formatNodeTest(test))
}

// findElementDecl finds an element declaration in an element's content model.
func findElementDecl(schema *parser.Schema, elementDecl *types.ElementDecl, test xpath.NodeTest) (*types.ElementDecl, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
	elementType := typeops.ResolveTypeReference(schema, elementDecl.Type, typeops.TypeReferenceMustExist)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	ct, ok := elementType.(*types.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}

	return findElementDeclInContent(ct.Content(), test)
}

// findElementDeclInContent searches for an element declaration in a content model.
func findElementDeclInContent(content types.Content, test xpath.NodeTest) (*types.ElementDecl, error) {
	switch content.(type) {
	case *types.SimpleContent:
		return nil, fmt.Errorf("element '%s' not found in simple content", formatNodeTest(test))
	case *types.EmptyContent:
		return nil, fmt.Errorf("element '%s' not found in empty content", formatNodeTest(test))
	}

	var result *types.ElementDecl
	var resultErr error

	err := traversal.WalkContentParticles(content, func(particle types.Particle) error {
		found, err := findElementDeclInParticle(particle, test)
		if err == nil && found != nil {
			result = found
			return nil
		}
		if resultErr == nil {
			resultErr = err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	if resultErr != nil {
		return nil, resultErr
	}
	return nil, fmt.Errorf("element '%s' not found in content model", formatNodeTest(test))
}

// findElementDeclInParticle searches for an element declaration in a particle tree.
func findElementDeclInParticle(particle types.Particle, test xpath.NodeTest) (*types.ElementDecl, error) {
	switch p := particle.(type) {
	case *types.ElementDecl:
		if nodeTestMatchesQName(test, p.Name) {
			return p, nil
		}
	case *types.ModelGroup:
		var unresolvedErr error
		for _, childParticle := range p.Particles {
			if decl, err := findElementDeclInParticle(childParticle, test); err == nil {
				return decl, nil
			} else if errors.Is(err, ErrXPathUnresolvable) && unresolvedErr == nil {
				unresolvedErr = err
			}
		}
		if unresolvedErr != nil {
			return nil, unresolvedErr
		}
		return nil, fmt.Errorf("element '%s' not found in model group", formatNodeTest(test))
	case *types.AnyElement:
		return nil, fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}

	return nil, fmt.Errorf("element '%s' not found in particle", formatNodeTest(test))
}
