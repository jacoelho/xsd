package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

func resolveAttributeType(schema *parser.Schema, typ model.Type, message string, test runtime.NodeTest) (model.Type, error) {
	resolvedType := parser.ResolveTypeReference(schema, typ)
	if resolvedType == nil {
		return nil, fmt.Errorf(message, formatNodeTest(test))
	}
	return resolvedType, nil
}

func findAttributeType(schema *parser.Schema, elementDecl *model.ElementDecl, test runtime.NodeTest) (model.Type, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
	ct, err := resolveIdentityElementComplexType(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	attrType, ok, err := findAttributeTypeInComplexType(schema, ct, test)
	if err != nil {
		return nil, err
	}
	if ok {
		return attrType, nil
	}
	return nil, fmt.Errorf("%w: attribute '%s' not found in element type", ErrXPathUnresolvable, formatNodeTest(test))
}

func findAttributeTypeInComplexType(schema *parser.Schema, ct *model.ComplexType, test runtime.NodeTest) (model.Type, bool, error) {
	for _, attrUse := range ct.Attributes() {
		if nodeTestMatchesQName(test, attrUse.Name) {
			resolvedType, err := resolveAttributeType(schema, attrUse.Type, "cannot resolve attribute type for '%s'", test)
			if err != nil {
				return nil, false, err
			}
			return resolvedType, true, nil
		}
	}
	for _, attrGroupQName := range ct.AttrGroups {
		attrType, ok, err := findAttributeTypeInAttributeGroup(schema, attrGroupQName, test, false)
		if ok || err != nil {
			return attrType, ok, err
		}
	}
	return nil, false, nil
}

func findAttributeTypeInAttributeGroup(schema *parser.Schema, name model.QName, test runtime.NodeTest, nested bool) (model.Type, bool, error) {
	attrGroup, ok := schema.AttributeGroups[name]
	if !ok {
		return nil, false, nil
	}
	for _, attr := range attrGroup.Attributes {
		if nodeTestMatchesQName(test, attr.Name) {
			message := "cannot resolve attribute type for '%s' in attribute group"
			if !nested {
				message = "cannot resolve attribute type for '%s' in attribute group"
			}
			resolvedType, err := resolveAttributeType(schema, attr.Type, message, test)
			if err != nil {
				return nil, false, err
			}
			return resolvedType, true, nil
		}
	}
	for _, nestedGroup := range attrGroup.AttrGroups {
		attrType, ok, err := findAttributeTypeInNestedAttributeGroup(schema, nestedGroup, test)
		if ok || err != nil {
			return attrType, ok, err
		}
	}
	return nil, false, nil
}

func findAttributeTypeInNestedAttributeGroup(schema *parser.Schema, name model.QName, test runtime.NodeTest) (model.Type, bool, error) {
	attrGroup, ok := schema.AttributeGroups[name]
	if !ok {
		return nil, false, nil
	}
	for _, attr := range attrGroup.Attributes {
		if nodeTestMatchesQName(test, attr.Name) {
			resolvedType, err := resolveAttributeType(schema, attr.Type, "cannot resolve attribute type for '%s' in nested attribute group", test)
			if err != nil {
				return nil, false, err
			}
			return resolvedType, true, nil
		}
	}
	return nil, false, nil
}

func findAttributeTypeDescendant(schema *parser.Schema, elementDecl *model.ElementDecl, test runtime.NodeTest) (model.Type, error) {
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
	ct, err := resolveIdentityElementComplexType(schema, elementDecl)
	if err != nil {
		return nil, err
	}

	state := newIdentitySearchState()
	state.visitedTypes[ct] = true

	var result model.Type
	visit := func(elem *model.ElementDecl) bool {
		attrType, err := findAttributeType(schema, elem, test)
		if err != nil {
			return false
		}
		result = attrType
		return true
	}
	searchParticle := func(particle model.Particle) (bool, bool) {
		if particle == nil {
			return false, false
		}
		return searchIdentityParticle(schema, particle, identitySearchDescendant, state, visit)
	}
	wildcardErr := func() error {
		return fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}

	switch c := ct.Content().(type) {
	case *model.ElementContent:
		found, unresolved := searchParticle(c.Particle)
		if found {
			return result, nil
		}
		if unresolved {
			return nil, wildcardErr()
		}

	case *model.SimpleContent:
		return nil, fmt.Errorf("attribute '%s' not found in simple content", formatNodeTest(test))

	case *model.ComplexContent:
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

	case *model.EmptyContent:
		return nil, fmt.Errorf("attribute '%s' not found in empty content", formatNodeTest(test))
	}

	return nil, fmt.Errorf("attribute '%s' not found in content model", formatNodeTest(test))
}
