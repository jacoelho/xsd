package fieldresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/xpath"
)

func resolvePathElementDecl(schema *parser.Schema, startDecl *model.ElementDecl, steps []xpath.Step) (*model.ElementDecl, error) {
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
func findElementDeclDescendant(schema *parser.Schema, elementDecl *model.ElementDecl, test xpath.NodeTest) (*model.ElementDecl, error) {
	ct, err := resolveComplexTypeForElementSearch(schema, elementDecl)
	if err != nil {
		return nil, err
	}

	visited := map[*model.ComplexType]struct{}{
		ct: {},
	}
	decl, err := findElementDeclInContentDescendant(schema, ct.Content(), test, visited)
	if err != nil && ct.Abstract {
		return nil, fmt.Errorf("%w: %w", ErrXPathUnresolvable, err)
	}
	return decl, err
}

// findElementDeclInContentDescendant searches for an element declaration at any depth in content.
func findElementDeclInContentDescendant(schema *parser.Schema, content model.Content, test xpath.NodeTest, visited map[*model.ComplexType]struct{}) (*model.ElementDecl, error) {
	return findElementDeclInContentWithMode(schema, content, test, elementPathSearchDescendant, visited)
}

// findElementDecl finds an element declaration in an element's content model.
func findElementDecl(schema *parser.Schema, elementDecl *model.ElementDecl, test xpath.NodeTest) (*model.ElementDecl, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}
	ct, err := resolveComplexTypeForElementSearch(schema, elementDecl)
	if err != nil {
		return nil, err
	}

	return findElementDeclInContent(ct.Content(), test)
}

// findElementDeclInContent searches for an element declaration in a content model.
func findElementDeclInContent(content model.Content, test xpath.NodeTest) (*model.ElementDecl, error) {
	return findElementDeclInContentWithMode(nil, content, test, elementPathSearchDirect, nil)
}

type elementPathSearchMode uint8

const (
	elementPathSearchDirect elementPathSearchMode = iota
	elementPathSearchDescendant
)

func resolveComplexTypeForElementSearch(schema *parser.Schema, elementDecl *model.ElementDecl) (*model.ComplexType, error) {
	elementDecl = resolveElementReference(schema, elementDecl)
	elementType := typeresolve.ResolveTypeReference(schema, elementDecl.Type, typeresolve.TypeReferenceMustExist)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}
	ct, ok := elementType.(*model.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}
	return ct, nil
}

func findElementDeclInContentWithMode(schema *parser.Schema, content model.Content, test xpath.NodeTest, mode elementPathSearchMode, visited map[*model.ComplexType]struct{}) (*model.ElementDecl, error) {
	switch c := content.(type) {
	case *model.ElementContent:
		if c.Particle != nil {
			return findElementDeclInParticleWithMode(schema, c.Particle, test, mode, visited)
		}
	case *model.SimpleContent:
		return nil, fmt.Errorf("element '%s' not found in simple content", formatNodeTest(test))
	case *model.ComplexContent:
		switch mode {
		case elementPathSearchDescendant:
			if c.Extension != nil && c.Extension.Particle != nil {
				if decl, err := findElementDeclInParticleWithMode(schema, c.Extension.Particle, test, mode, visited); err == nil {
					return decl, nil
				}
			}
			if c.Restriction != nil && c.Restriction.Particle != nil {
				return findElementDeclInParticleWithMode(schema, c.Restriction.Particle, test, mode, visited)
			}
		default:
			var resultErr error
			if c.Extension != nil && c.Extension.Particle != nil {
				if decl, err := findElementDeclInParticleWithMode(schema, c.Extension.Particle, test, mode, visited); err == nil {
					return decl, nil
				} else {
					resultErr = err
				}
			}
			if c.Restriction != nil && c.Restriction.Particle != nil {
				if decl, err := findElementDeclInParticleWithMode(schema, c.Restriction.Particle, test, mode, visited); err == nil {
					return decl, nil
				} else if resultErr == nil {
					resultErr = err
				}
			}
			if resultErr != nil {
				return nil, resultErr
			}
		}
	case *model.EmptyContent:
		return nil, fmt.Errorf("element '%s' not found in empty content", formatNodeTest(test))
	}

	return nil, fmt.Errorf("element '%s' not found in content model", formatNodeTest(test))
}

func findElementDeclInParticleWithMode(schema *parser.Schema, particle model.Particle, test xpath.NodeTest, mode elementPathSearchMode, visited map[*model.ComplexType]struct{}) (*model.ElementDecl, error) {
	switch p := particle.(type) {
	case *model.ElementDecl:
		elem := p
		if mode == elementPathSearchDescendant {
			elem = resolveElementReference(schema, p)
		}
		if nodeTestMatchesQName(test, elem.Name) {
			return elem, nil
		}
		if mode == elementPathSearchDescendant && elem.Type != nil {
			if visited == nil {
				visited = make(map[*model.ComplexType]struct{})
			}
			if resolvedType := typeresolve.ResolveTypeReference(schema, elem.Type, typeresolve.TypeReferenceMustExist); resolvedType != nil {
				if ct, ok := resolvedType.(*model.ComplexType); ok {
					if _, seen := visited[ct]; !seen {
						visited[ct] = struct{}{}
						if decl, err := findElementDeclInContentWithMode(schema, ct.Content(), test, mode, visited); err == nil {
							return decl, nil
						}
					}
				}
			}
		}
	case *model.ModelGroup:
		var unresolvedErr error
		for _, childParticle := range p.Particles {
			if decl, err := findElementDeclInParticleWithMode(schema, childParticle, test, mode, visited); err == nil {
				return decl, nil
			} else if errors.Is(err, ErrXPathUnresolvable) && unresolvedErr == nil {
				unresolvedErr = err
			}
		}
		if unresolvedErr != nil {
			return nil, unresolvedErr
		}
		return nil, fmt.Errorf("element '%s' not found in model group", formatNodeTest(test))
	case *model.AnyElement:
		return nil, fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}

	return nil, fmt.Errorf("element '%s' not found in particle", formatNodeTest(test))
}
