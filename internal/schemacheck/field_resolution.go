package schemacheck

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

// ErrFieldSelectsComplexContent indicates a field XPath selects an element with
// complex content, which is invalid per XSD spec Section 13.2.
var ErrFieldSelectsComplexContent = errors.New("field selects element with complex content")

func parseXPathPath(expr string, nsContext map[string]string, policy xpath.AttributePolicy) (xpath.Path, error) {
	parsed, err := xpath.Parse(expr, nsContext, policy)
	if err != nil {
		return xpath.Path{}, err
	}
	if len(parsed.Paths) == 0 {
		return xpath.Path{}, fmt.Errorf("xpath contains no paths")
	}
	return parsed.Paths[0], nil
}

func isWildcardNodeTest(test xpath.NodeTest) bool {
	return test.Any || test.Local == "*"
}

func nodeTestMatchesQName(test xpath.NodeTest, name types.QName) bool {
	if test.Any {
		return true
	}
	if test.Local != "*" && name.Local != test.Local {
		return false
	}
	if test.NamespaceSpecified && name.Namespace != test.Namespace {
		return false
	}
	return true
}

func formatNodeTest(test xpath.NodeTest) string {
	if isWildcardNodeTest(test) {
		return "*"
	}
	if !test.NamespaceSpecified || test.Namespace.IsEmpty() {
		return test.Local
	}
	return "{" + test.Namespace.String() + "}" + test.Local
}

func isDescendantOnlySteps(steps []xpath.Step) bool {
	if len(steps) == 0 {
		return false
	}
	sawDescendant := false
	for _, step := range steps {
		switch step.Axis {
		case xpath.AxisDescendantOrSelf:
			if !step.Test.Any {
				return false
			}
			sawDescendant = true
		case xpath.AxisSelf:
			if !step.Test.Any {
				return false
			}
		default:
			return false
		}
	}
	return sawDescendant
}

// resolveFieldType resolves the type of a field XPath expression.
// Returns the type of the attribute or element selected by the field.
// The selectorXPath is used to determine the context element (the element selected by the selector).
func resolveFieldType(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (types.Type, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}

	fieldPath, err := parseXPathPath(field.XPath, nsContext, xpath.AttributesAllowed)
	if err != nil {
		return nil, err
	}

	// first, resolve the selector to find the selected element type
	// fields are resolved relative to the element selected by the selector
	selectedElementType, err := resolveSelectorElementType(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, fmt.Errorf("resolve selector '%s': %w", selectorXPath, err)
	}

	// this allows us to reuse findAttributeType and findElementType
	selectedElementDecl := &types.ElementDecl{
		Type: selectedElementType,
	}

	if fieldPath.Attribute != nil && isDescendantOnlySteps(fieldPath.Steps) {
		attrType, attrErr := findAttributeTypeDescendant(schema, selectedElementDecl, *fieldPath.Attribute)
		if attrErr != nil {
			return nil, fmt.Errorf("resolve attribute field '%s': %w", formatNodeTest(*fieldPath.Attribute), attrErr)
		}
		return attrType, nil
	}

	elementDecl, err := resolvePathElementDecl(schema, selectedElementDecl, fieldPath.Steps)
	if err != nil {
		return nil, fmt.Errorf("resolve field path '%s': %w", field.XPath, err)
	}

	if fieldPath.Attribute != nil {
		attrType, err := findAttributeType(schema, elementDecl, *fieldPath.Attribute)
		if err != nil {
			return nil, fmt.Errorf("resolve attribute field '%s': %w", formatNodeTest(*fieldPath.Attribute), err)
		}
		return attrType, nil
	}

	elementType := resolveTypeForValidation(schema, elementDecl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	// verify element has simple type
	if ct, ok := elementType.(*types.ComplexType); ok {
		// check if it's simple content (has simple type base)
		if _, ok := ct.Content().(*types.SimpleContent); ok {
			baseType := ct.BaseType()
			if baseType != nil {
				return baseType, nil
			}
		}
		// complex types without simple content can't be used as field values
		return nil, ErrFieldSelectsComplexContent
	}

	return elementType, nil
}

// resolveFieldElementDecl resolves a field XPath to the selected element declaration.
// Returns nil if the field selects an attribute.
func resolveFieldElementDecl(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (*types.ElementDecl, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}

	fieldPath, err := parseXPathPath(field.XPath, nsContext, xpath.AttributesAllowed)
	if err != nil {
		return nil, err
	}
	if fieldPath.Attribute != nil {
		return nil, fmt.Errorf("field xpath selects attribute: %s", field.XPath)
	}

	selectedElementDecl, err := resolveSelectorElementDecl(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, fmt.Errorf("resolve selector '%s': %w", selectorXPath, err)
	}
	if selectedElementDecl == nil {
		return nil, fmt.Errorf("cannot resolve selector element")
	}

	return resolvePathElementDecl(schema, selectedElementDecl, fieldPath.Steps)
}

// resolveSelectorElementType resolves the type of the element selected by the selector XPath.
func resolveSelectorElementType(schema *parser.Schema, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (types.Type, error) {
	path, err := parseXPathPath(selectorXPath, nsContext, xpath.AttributesDisallowed)
	if err != nil {
		return nil, err
	}
	if path.Attribute != nil {
		return nil, fmt.Errorf("selector xpath cannot select attributes: %s", selectorXPath)
	}

	elementDecl, err := resolvePathElementDecl(schema, constraintElement, path.Steps)
	if err != nil {
		return nil, err
	}

	elementType := resolveTypeForValidation(schema, elementDecl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve constraint element type")
	}
	return elementType, nil
}

// resolveSelectorElementDecl resolves the element declaration selected by a selector XPath.
func resolveSelectorElementDecl(schema *parser.Schema, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (*types.ElementDecl, error) {
	path, err := parseXPathPath(selectorXPath, nsContext, xpath.AttributesDisallowed)
	if err != nil {
		return nil, err
	}
	if path.Attribute != nil {
		return nil, fmt.Errorf("selector xpath cannot select attributes: %s", selectorXPath)
	}
	return resolvePathElementDecl(schema, constraintElement, path.Steps)
}

func resolvePathElementDecl(schema *parser.Schema, startDecl *types.ElementDecl, steps []xpath.Step) (*types.ElementDecl, error) {
	current := startDecl
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
				return nil, fmt.Errorf("xpath step is missing a node test")
			}
			if !step.Test.Any && current != nil && !nodeTestMatchesQName(step.Test, current.Name) {
				return nil, fmt.Errorf("xpath self step does not match current element")
			}
			continue
		case xpath.AxisChild:
			// handled below.
		default:
			return nil, fmt.Errorf("xpath uses disallowed axis")
		}

		if isWildcardNodeTest(step.Test) {
			return nil, fmt.Errorf("cannot resolve element declaration for wildcard")
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
	}

	if descendantNext {
		return nil, fmt.Errorf("xpath step is missing a node test")
	}
	if current == nil {
		return nil, fmt.Errorf("cannot resolve element declaration")
	}
	return current, nil
}

// findElementTypeDescendant searches for an element at any depth in the content model
// This is used for descendant axis selectors
func findElementTypeDescendant(schema *parser.Schema, elementDecl *types.ElementDecl, test xpath.NodeTest) (types.Type, error) {
	// resolve element's type
	elementType := resolveTypeForValidation(schema, elementDecl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	ct, ok := elementType.(*types.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}

	// search content model recursively for element declaration
	visited := map[*types.ComplexType]struct{}{
		ct: {},
	}
	return findElementInContentDescendant(schema, ct.Content(), test, visited)
}

// findElementDeclDescendant searches for an element declaration at any depth in the content model.
func findElementDeclDescendant(schema *parser.Schema, elementDecl *types.ElementDecl, test xpath.NodeTest) (*types.ElementDecl, error) {
	elementType := resolveTypeForValidation(schema, elementDecl.Type)
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
	return findElementDeclInContentDescendant(schema, ct.Content(), test, visited)
}

// findElementInContentDescendant searches for an element at any depth in content
func findElementInContentDescendant(schema *parser.Schema, content types.Content, test xpath.NodeTest, visited map[*types.ComplexType]struct{}) (types.Type, error) {
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			return findElementInParticleDescendant(schema, c.Particle, test, visited)
		}
	case *types.SimpleContent:
		return nil, fmt.Errorf("element '%s' not found in simple content", formatNodeTest(test))
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			if typ, err := findElementInParticleDescendant(schema, c.Extension.Particle, test, visited); err == nil {
				return typ, nil
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			return findElementInParticleDescendant(schema, c.Restriction.Particle, test, visited)
		}
	case *types.EmptyContent:
		return nil, fmt.Errorf("element '%s' not found in empty content", formatNodeTest(test))
	}

	return nil, fmt.Errorf("element '%s' not found in content model", formatNodeTest(test))
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

// findElementInParticleDescendant searches for an element at any depth in a particle tree
// This recursively searches through all nested particles
func findElementInParticleDescendant(schema *parser.Schema, particle types.Particle, test xpath.NodeTest, visited map[*types.ComplexType]struct{}) (types.Type, error) {
	switch p := particle.(type) {
	case *types.ElementDecl:
		// found an element declaration
		if nodeTestMatchesQName(test, p.Name) {
			resolvedType := resolveTypeForValidation(schema, p.Type)
			if resolvedType == nil {
				return nil, fmt.Errorf("cannot resolve element type for '%s'", formatNodeTest(test))
			}
			return resolvedType, nil
		}
		// even if name doesn't match, if this element has complex type, search its content
		if p.Type != nil {
			if resolvedType := resolveTypeForValidation(schema, p.Type); resolvedType != nil {
				if ct, ok := resolvedType.(*types.ComplexType); ok {
					if _, seen := visited[ct]; !seen {
						visited[ct] = struct{}{}
						if typ, err := findElementInContentDescendant(schema, ct.Content(), test, visited); err == nil {
							return typ, nil
						}
					}
				}
			}
		}
	case *types.ModelGroup:
		// search in model group particles recursively
		for _, childParticle := range p.Particles {
			if typ, err := findElementInParticleDescendant(schema, childParticle, test, visited); err == nil {
				return typ, nil
			}
		}
		return nil, fmt.Errorf("element '%s' not found in model group", formatNodeTest(test))
	case *types.AnyElement:
		// wildcard - cannot determine specific element type
		return nil, fmt.Errorf("cannot resolve element type for wildcard")
	}

	return nil, fmt.Errorf("element '%s' not found in particle", formatNodeTest(test))
}

// findElementDeclInParticleDescendant searches for an element declaration at any depth in a particle tree.
func findElementDeclInParticleDescendant(schema *parser.Schema, particle types.Particle, test xpath.NodeTest, visited map[*types.ComplexType]struct{}) (*types.ElementDecl, error) {
	switch p := particle.(type) {
	case *types.ElementDecl:
		if nodeTestMatchesQName(test, p.Name) {
			return p, nil
		}
		if p.Type != nil {
			if resolvedType := resolveTypeForValidation(schema, p.Type); resolvedType != nil {
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
		for _, childParticle := range p.Particles {
			if decl, err := findElementDeclInParticleDescendant(schema, childParticle, test, visited); err == nil {
				return decl, nil
			}
		}
		return nil, fmt.Errorf("element '%s' not found in model group", formatNodeTest(test))
	case *types.AnyElement:
		return nil, fmt.Errorf("cannot resolve element declaration for wildcard")
	}

	return nil, fmt.Errorf("element '%s' not found in particle", formatNodeTest(test))
}

// findAttributeType finds the type of an attribute in an element's type.
func findAttributeType(schema *parser.Schema, elementDecl *types.ElementDecl, test xpath.NodeTest) (types.Type, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("cannot resolve attribute type for wildcard")
	}
	// resolve element's type
	elementType := resolveTypeForValidation(schema, elementDecl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	ct, ok := elementType.(*types.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}

	// search attribute uses
	for _, attrUse := range ct.Attributes() {
		if nodeTestMatchesQName(test, attrUse.Name) {
			resolvedType := resolveTypeForValidation(schema, attrUse.Type)
			if resolvedType == nil {
				return nil, fmt.Errorf("cannot resolve attribute type for '%s'", formatNodeTest(test))
			}
			return resolvedType, nil
		}
	}

	// search attribute groups
	for _, attrGroupQName := range ct.AttrGroups {
		// look up attribute group in schema
		if attrGroup, ok := schema.AttributeGroups[attrGroupQName]; ok {
			for _, attr := range attrGroup.Attributes {
				if nodeTestMatchesQName(test, attr.Name) {
					resolvedType := resolveTypeForValidation(schema, attr.Type)
					if resolvedType == nil {
						return nil, fmt.Errorf("cannot resolve attribute type for '%s' in attribute group", formatNodeTest(test))
					}
					return resolvedType, nil
				}
			}
			// recursively search nested attribute groups
			for _, nestedAttrGroupQName := range attrGroup.AttrGroups {
				if nestedAttrGroup, ok := schema.AttributeGroups[nestedAttrGroupQName]; ok {
					for _, attr := range nestedAttrGroup.Attributes {
						if nodeTestMatchesQName(test, attr.Name) {
							resolvedType := resolveTypeForValidation(schema, attr.Type)
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

	// note: anyAttribute wildcard cannot be used to resolve specific attribute types.
	// fields must reference declared attributes, not wildcard attributes (per XSD 1.0 spec).

	return nil, fmt.Errorf("attribute '%s' not found in element type", formatNodeTest(test))
}

// findAttributeTypeDescendant searches for an attribute at any depth in the content model.
func findAttributeTypeDescendant(schema *parser.Schema, elementDecl *types.ElementDecl, test xpath.NodeTest) (types.Type, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("cannot resolve attribute type for wildcard")
	}
	if elementDecl == nil {
		return nil, fmt.Errorf("cannot resolve attribute type without element declaration")
	}
	if attrType, err := findAttributeType(schema, elementDecl, test); err == nil {
		return attrType, nil
	}

	elementType := resolveTypeForValidation(schema, elementDecl.Type)
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
	return findAttributeTypeInContentDescendant(schema, ct.Content(), test, visited)
}

// findAttributeTypeInContentDescendant searches for an attribute at any depth in content.
func findAttributeTypeInContentDescendant(schema *parser.Schema, content types.Content, test xpath.NodeTest, visited map[*types.ComplexType]struct{}) (types.Type, error) {
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			return findAttributeTypeInParticleDescendant(schema, c.Particle, test, visited)
		}
	case *types.SimpleContent:
		return nil, fmt.Errorf("attribute '%s' not found in simple content", formatNodeTest(test))
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			if typ, err := findAttributeTypeInParticleDescendant(schema, c.Extension.Particle, test, visited); err == nil {
				return typ, nil
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			return findAttributeTypeInParticleDescendant(schema, c.Restriction.Particle, test, visited)
		}
	case *types.EmptyContent:
		return nil, fmt.Errorf("attribute '%s' not found in empty content", formatNodeTest(test))
	}

	return nil, fmt.Errorf("attribute '%s' not found in content model", formatNodeTest(test))
}

// findAttributeTypeInParticleDescendant searches for an attribute at any depth in a particle tree.
func findAttributeTypeInParticleDescendant(schema *parser.Schema, particle types.Particle, test xpath.NodeTest, visited map[*types.ComplexType]struct{}) (types.Type, error) {
	switch p := particle.(type) {
	case *types.ElementDecl:
		if attrType, err := findAttributeType(schema, p, test); err == nil {
			return attrType, nil
		}
		if p.Type != nil {
			if resolvedType := resolveTypeForValidation(schema, p.Type); resolvedType != nil {
				if ct, ok := resolvedType.(*types.ComplexType); ok {
					if _, seen := visited[ct]; !seen {
						visited[ct] = struct{}{}
						if typ, err := findAttributeTypeInContentDescendant(schema, ct.Content(), test, visited); err == nil {
							return typ, nil
						}
					}
				}
			}
		}
	case *types.ModelGroup:
		for _, childParticle := range p.Particles {
			if typ, err := findAttributeTypeInParticleDescendant(schema, childParticle, test, visited); err == nil {
				return typ, nil
			}
		}
		return nil, fmt.Errorf("attribute '%s' not found in model group", formatNodeTest(test))
	case *types.AnyElement:
		return nil, fmt.Errorf("cannot resolve attribute type for wildcard")
	}

	return nil, fmt.Errorf("attribute '%s' not found in particle", formatNodeTest(test))
}

// resolveFieldElementDeclPath resolves a field element path to an element declaration.
func resolveFieldElementDeclPath(schema *parser.Schema, elementDecl *types.ElementDecl, elementPath string, nsContext map[string]string) (*types.ElementDecl, error) {
	path, err := parseXPathPath(elementPath, nsContext, xpath.AttributesDisallowed)
	if err != nil {
		return nil, err
	}
	if path.Attribute != nil {
		return nil, fmt.Errorf("field xpath cannot select attributes: %s", elementPath)
	}
	return resolvePathElementDecl(schema, elementDecl, path.Steps)
}

// resolveFieldElementType resolves the element type from a field element path.
// Handles both simple element names and nested paths like "part" or "orders/order".
func resolveFieldElementType(schema *parser.Schema, elementDecl *types.ElementDecl, elementPath string, nsContext map[string]string) (types.Type, error) {
	decl, err := resolveFieldElementDeclPath(schema, elementDecl, elementPath, nsContext)
	if err != nil {
		return nil, err
	}
	elementType := resolveTypeForValidation(schema, decl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}
	return elementType, nil
}

// findElementDecl finds an element declaration in an element's content model.
func findElementDecl(schema *parser.Schema, elementDecl *types.ElementDecl, test xpath.NodeTest) (*types.ElementDecl, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("cannot resolve element declaration for wildcard")
	}
	elementType := resolveTypeForValidation(schema, elementDecl.Type)
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

	err := WalkContentParticles(content, func(particle types.Particle) error {
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
		for _, childParticle := range p.Particles {
			if decl, err := findElementDeclInParticle(childParticle, test); err == nil {
				return decl, nil
			}
		}
		return nil, fmt.Errorf("element '%s' not found in model group", formatNodeTest(test))
	case *types.AnyElement:
		return nil, fmt.Errorf("cannot resolve element declaration for wildcard")
	}

	return nil, fmt.Errorf("element '%s' not found in particle", formatNodeTest(test))
}
