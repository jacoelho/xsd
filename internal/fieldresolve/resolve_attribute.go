package fieldresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/xpath"
)

// findAttributeType finds the type of an attribute in an element's type.
func findAttributeType(schema *parser.Schema, elementDecl *model.ElementDecl, test xpath.NodeTest) (model.Type, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
	elementType := typeops.ResolveTypeReference(schema, elementDecl.Type, typeops.TypeReferenceMustExist)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	ct, ok := elementType.(*model.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}

	for _, attrUse := range ct.Attributes() {
		if nodeTestMatchesQName(test, attrUse.Name) {
			resolvedType := typeops.ResolveTypeReference(schema, attrUse.Type, typeops.TypeReferenceMustExist)
			if resolvedType == nil {
				return nil, fmt.Errorf("cannot resolve attribute type for '%s'", formatNodeTest(test))
			}
			return resolvedType, nil
		}
	}

	for _, attrGroupQName := range ct.AttrGroups {
		if attrGroup, ok := schema.AttributeGroups[attrGroupQName]; ok {
			for _, attr := range attrGroup.Attributes {
				if nodeTestMatchesQName(test, attr.Name) {
					resolvedType := typeops.ResolveTypeReference(schema, attr.Type, typeops.TypeReferenceMustExist)
					if resolvedType == nil {
						return nil, fmt.Errorf("cannot resolve attribute type for '%s' in attribute group", formatNodeTest(test))
					}
					return resolvedType, nil
				}
			}
			for _, nestedAttrGroupQName := range attrGroup.AttrGroups {
				if nestedAttrGroup, ok := schema.AttributeGroups[nestedAttrGroupQName]; ok {
					for _, attr := range nestedAttrGroup.Attributes {
						if nodeTestMatchesQName(test, attr.Name) {
							resolvedType := typeops.ResolveTypeReference(schema, attr.Type, typeops.TypeReferenceMustExist)
							if resolvedType == nil {
								return nil, fmt.Errorf("cannot resolve attribute type for '%s' in nested attribute group", formatNodeTest(test))
							}
							return resolvedType, nil
						}
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("%w: attribute '%s' not found in element type", ErrXPathUnresolvable, formatNodeTest(test))
}

// findAttributeTypeDescendant searches for an attribute at any depth in the content model.
func findAttributeTypeDescendant(schema *parser.Schema, elementDecl *model.ElementDecl, test xpath.NodeTest) (model.Type, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
	if elementDecl == nil {
		return nil, fmt.Errorf("cannot resolve attribute type without element declaration")
	}
	if attrType, err := findAttributeType(schema, elementDecl, test); err == nil {
		return attrType, nil
	}

	elementType := typeops.ResolveTypeReference(schema, elementDecl.Type, typeops.TypeReferenceMustExist)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	ct, ok := elementType.(*model.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}

	visited := map[*model.ComplexType]struct{}{
		ct: {},
	}
	return findAttributeTypeInContentDescendant(schema, ct.Content(), test, visited)
}

// findAttributeTypeInContentDescendant searches for an attribute at any depth in content.
func findAttributeTypeInContentDescendant(schema *parser.Schema, content model.Content, test xpath.NodeTest, visited map[*model.ComplexType]struct{}) (model.Type, error) {
	switch c := content.(type) {
	case *model.ElementContent:
		if c.Particle != nil {
			return findAttributeTypeInParticleDescendant(schema, c.Particle, test, visited)
		}
	case *model.SimpleContent:
		return nil, fmt.Errorf("attribute '%s' not found in simple content", formatNodeTest(test))
	case *model.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			if typ, err := findAttributeTypeInParticleDescendant(schema, c.Extension.Particle, test, visited); err == nil {
				return typ, nil
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			return findAttributeTypeInParticleDescendant(schema, c.Restriction.Particle, test, visited)
		}
	case *model.EmptyContent:
		return nil, fmt.Errorf("attribute '%s' not found in empty content", formatNodeTest(test))
	}

	return nil, fmt.Errorf("attribute '%s' not found in content model", formatNodeTest(test))
}

// findAttributeTypeInParticleDescendant searches for an attribute at any depth in a particle tree.
func findAttributeTypeInParticleDescendant(schema *parser.Schema, particle model.Particle, test xpath.NodeTest, visited map[*model.ComplexType]struct{}) (model.Type, error) {
	switch p := particle.(type) {
	case *model.ElementDecl:
		elem := resolveElementReference(schema, p)
		if attrType, err := findAttributeType(schema, elem, test); err == nil {
			return attrType, nil
		}
		if elem.Type != nil {
			if resolvedType := typeops.ResolveTypeReference(schema, elem.Type, typeops.TypeReferenceMustExist); resolvedType != nil {
				if ct, ok := resolvedType.(*model.ComplexType); ok {
					if _, seen := visited[ct]; !seen {
						visited[ct] = struct{}{}
						if typ, err := findAttributeTypeInContentDescendant(schema, ct.Content(), test, visited); err == nil {
							return typ, nil
						}
					}
				}
			}
		}
	case *model.ModelGroup:
		var unresolvedErr error
		for _, childParticle := range p.Particles {
			if typ, err := findAttributeTypeInParticleDescendant(schema, childParticle, test, visited); err == nil {
				return typ, nil
			} else if errors.Is(err, ErrXPathUnresolvable) && unresolvedErr == nil {
				unresolvedErr = err
			}
		}
		if unresolvedErr != nil {
			return nil, unresolvedErr
		}
		return nil, fmt.Errorf("attribute '%s' not found in model group", formatNodeTest(test))
	case *model.AnyElement:
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}

	return nil, fmt.Errorf("attribute '%s' not found in particle", formatNodeTest(test))
}
