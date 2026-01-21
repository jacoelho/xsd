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

// ErrXPathUnresolvable indicates a selector or field XPath cannot be resolved
// statically, such as when wildcard steps are present.
var ErrXPathUnresolvable = errors.New("xpath cannot be resolved statically")

func parseXPathExpression(expr string, nsContext map[string]string, policy xpath.AttributePolicy) (xpath.Expression, error) {
	parsed, err := xpath.Parse(expr, nsContext, policy)
	if err != nil {
		return xpath.Expression{}, err
	}
	if len(parsed.Paths) == 0 {
		return xpath.Expression{}, fmt.Errorf("xpath contains no paths")
	}
	return parsed, nil
}

func parseXPathPath(expr string, nsContext map[string]string, policy xpath.AttributePolicy) (xpath.Path, error) {
	parsed, err := parseXPathExpression(expr, nsContext, policy)
	if err != nil {
		return xpath.Path{}, err
	}
	if len(parsed.Paths) != 1 {
		return xpath.Path{}, fmt.Errorf("xpath contains %d paths", len(parsed.Paths))
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

func resolveElementReference(schema *parser.Schema, decl *types.ElementDecl) *types.ElementDecl {
	if decl == nil || !decl.IsReference || schema == nil {
		return decl
	}
	if resolved, ok := schema.ElementDecls[decl.Name]; ok {
		return resolved
	}
	return decl
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

func fieldTypeName(typ types.Type) string {
	if typ == nil {
		return "<nil>"
	}
	name := typ.Name()
	if !name.IsZero() {
		return name.String()
	}
	return fmt.Sprintf("%T", typ)
}

func fieldTypeKey(typ types.Type) string {
	if typ == nil {
		return ""
	}
	name := typ.Name()
	if !name.IsZero() {
		return name.String()
	}
	return fmt.Sprintf("%T:%p", typ, typ)
}

func uniqueFieldTypes(values []types.Type) []types.Type {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	unique := make([]types.Type, 0, len(values))
	for _, typ := range values {
		if typ == nil {
			continue
		}
		key := fieldTypeKey(typ)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, typ)
	}
	return unique
}

func fieldTypesCompatible(a, b types.Type) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name() == b.Name() {
		return true
	}
	if types.IsDerivedFrom(a, b) || types.IsDerivedFrom(b, a) {
		return true
	}
	primA := a.PrimitiveType()
	primB := b.PrimitiveType()
	if primA != nil && primB != nil && primA.Name() == primB.Name() {
		return true
	}
	return false
}

func combineFieldTypes(fieldXPath string, values []types.Type) (types.Type, error) {
	unique := uniqueFieldTypes(values)
	if len(unique) == 0 {
		return nil, fmt.Errorf("field xpath '%s' resolves to no types", fieldXPath)
	}
	if len(unique) == 1 {
		return unique[0], nil
	}
	for i := 0; i < len(unique); i++ {
		for j := i + 1; j < len(unique); j++ {
			if !fieldTypesCompatible(unique[i], unique[j]) {
				return nil, fmt.Errorf("field xpath '%s' selects incompatible types '%s' and '%s'", fieldXPath, fieldTypeName(unique[i]), fieldTypeName(unique[j]))
			}
		}
	}
	return &types.SimpleType{
		Union:       &types.UnionType{},
		MemberTypes: unique,
	}, nil
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

	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, xpath.AttributesAllowed)
	if err != nil {
		return nil, err
	}

	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		if errors.Is(err, ErrXPathUnresolvable) {
			return nil, err
		}
		return nil, fmt.Errorf("resolve selector '%s': %w", selectorXPath, err)
	}

	hasUnion := len(fieldExpr.Paths) > 1 || len(selectorDecls) > 1
	var resolvedTypes []types.Type
	unresolved := false
	for _, selectorDecl := range selectorDecls {
		for _, fieldPath := range fieldExpr.Paths {
			typ, err := resolveFieldPathType(schema, selectorDecl, fieldPath)
			if err != nil {
				if hasUnion {
					unresolved = true
					continue
				}
				return nil, fmt.Errorf("resolve field xpath '%s': %w", field.XPath, err)
			}
			resolvedTypes = append(resolvedTypes, typ)
		}
	}

	if len(resolvedTypes) == 0 {
		if unresolved {
			return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
		}
		return nil, fmt.Errorf("field xpath '%s' resolves to no types", field.XPath)
	}
	combined, err := combineFieldTypes(field.XPath, resolvedTypes)
	if err != nil {
		if hasUnion {
			return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
		}
		return nil, err
	}
	if unresolved {
		return combined, fmt.Errorf("%w: field xpath '%s' contains wildcard branches", ErrXPathUnresolvable, field.XPath)
	}
	return combined, nil
}

// resolveFieldElementDecl resolves a field XPath to the selected element declaration.
// Returns nil if the field selects an attribute.
func resolveFieldElementDecl(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (*types.ElementDecl, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}

	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, xpath.AttributesAllowed)
	if err != nil {
		return nil, err
	}

	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, fmt.Errorf("resolve selector '%s': %w", selectorXPath, err)
	}

	var decls []*types.ElementDecl
	for _, selectorDecl := range selectorDecls {
		for pathIndex, fieldPath := range fieldExpr.Paths {
			if fieldPath.Attribute != nil {
				return nil, fmt.Errorf("field xpath selects attribute: %s", field.XPath)
			}
			decl, err := resolvePathElementDecl(schema, selectorDecl, fieldPath.Steps)
			if err != nil {
				return nil, fmt.Errorf("resolve field xpath '%s' branch %d: %w", field.XPath, pathIndex+1, err)
			}
			decls = append(decls, decl)
		}
	}

	unique := uniqueElementDecls(decls)
	if len(unique) != 1 {
		return nil, fmt.Errorf("field xpath '%s' resolves to multiple element declarations", field.XPath)
	}
	return unique[0], nil
}

// resolveSelectorElementType resolves the type of the element selected by the selector XPath.
func resolveSelectorElementType(schema *parser.Schema, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (types.Type, error) {
	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, err
	}

	var elementType types.Type
	for _, decl := range selectorDecls {
		resolved := resolveTypeForValidation(schema, decl.Type)
		if resolved == nil {
			return nil, fmt.Errorf("cannot resolve constraint element type")
		}
		if elementType == nil {
			elementType = resolved
			continue
		}
		if !elementTypesCompatible(elementType, resolved) {
			return nil, fmt.Errorf("selector xpath '%s' resolves to multiple element types", selectorXPath)
		}
	}
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve constraint element type")
	}
	return elementType, nil
}

// resolveSelectorElementDecl resolves the element declaration selected by a selector XPath.
func resolveSelectorElementDecl(schema *parser.Schema, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (*types.ElementDecl, error) {
	decls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, err
	}
	unique := uniqueElementDecls(decls)
	if len(unique) != 1 {
		return nil, fmt.Errorf("selector xpath '%s' resolves to multiple element declarations", selectorXPath)
	}
	return unique[0], nil
}

func resolveSelectorElementDecls(schema *parser.Schema, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) ([]*types.ElementDecl, error) {
	if constraintElement == nil {
		return nil, fmt.Errorf("constraint element is nil")
	}
	expr, err := parseXPathExpression(selectorXPath, nsContext, xpath.AttributesDisallowed)
	if err != nil {
		return nil, err
	}
	decls := make([]*types.ElementDecl, 0, len(expr.Paths))
	unresolved := false
	for i, path := range expr.Paths {
		if path.Attribute != nil {
			return nil, fmt.Errorf("selector xpath cannot select attributes: %s", selectorXPath)
		}
		decl, err := resolvePathElementDecl(schema, constraintElement, path.Steps)
		if err != nil {
			if errors.Is(err, ErrXPathUnresolvable) {
				unresolved = true
				continue
			}
			return nil, fmt.Errorf("resolve selector xpath '%s' branch %d: %w", selectorXPath, i+1, err)
		}
		decls = append(decls, decl)
	}
	if len(decls) == 0 {
		if unresolved {
			return nil, fmt.Errorf("%w: selector xpath '%s'", ErrXPathUnresolvable, selectorXPath)
		}
		return nil, fmt.Errorf("cannot resolve selector element")
	}
	return decls, nil
}

func uniqueElementDecls(decls []*types.ElementDecl) []*types.ElementDecl {
	if len(decls) == 0 {
		return nil
	}
	seen := make(map[types.QName]struct{}, len(decls))
	unique := make([]*types.ElementDecl, 0, len(decls))
	for _, decl := range decls {
		if decl == nil {
			continue
		}
		if _, ok := seen[decl.Name]; ok {
			continue
		}
		seen[decl.Name] = struct{}{}
		unique = append(unique, decl)
	}
	return unique
}

func resolveFieldPathType(schema *parser.Schema, selectedElementDecl *types.ElementDecl, fieldPath xpath.Path) (types.Type, error) {
	if selectedElementDecl == nil {
		return nil, fmt.Errorf("cannot resolve field without selector element")
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
		return nil, fmt.Errorf("resolve field path: %w", err)
	}
	elementDecl = resolveElementReference(schema, elementDecl)

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

	if ct, ok := elementType.(*types.ComplexType); ok {
		if _, ok := ct.Content().(*types.SimpleContent); ok {
			baseType := ct.BaseType()
			if baseType != nil {
				return baseType, nil
			}
		}
		return nil, ErrFieldSelectsComplexContent
	}

	return elementType, nil
}

func resolveFieldElementDecls(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) ([]*types.ElementDecl, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}
	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, xpath.AttributesAllowed)
	if err != nil {
		return nil, err
	}
	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, fmt.Errorf("resolve selector '%s': %w", selectorXPath, err)
	}
	var decls []*types.ElementDecl
	unresolved := false
	for _, selectorDecl := range selectorDecls {
		for pathIndex, fieldPath := range fieldExpr.Paths {
			if fieldPath.Attribute != nil {
				continue
			}
			decl, err := resolvePathElementDecl(schema, selectorDecl, fieldPath.Steps)
			if err != nil {
				if errors.Is(err, ErrXPathUnresolvable) {
					unresolved = true
					continue
				}
				return nil, fmt.Errorf("resolve field xpath '%s' branch %d: %w", field.XPath, pathIndex+1, err)
			}
			decls = append(decls, decl)
		}
	}
	unique := uniqueElementDecls(decls)
	if len(unique) == 0 && unresolved {
		return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
	}
	return unique, nil
}

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
			// handled below.
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

// findElementTypeDescendant searches for an element at any depth in the content model
// This is used for descendant axis selectors
func findElementTypeDescendant(schema *parser.Schema, elementDecl *types.ElementDecl, test xpath.NodeTest) (types.Type, error) {
	// resolve element's type
	elementDecl = resolveElementReference(schema, elementDecl)
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
	elementDecl = resolveElementReference(schema, elementDecl)
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
		elem := resolveElementReference(schema, p)
		// found an element declaration
		if nodeTestMatchesQName(test, elem.Name) {
			resolvedType := resolveTypeForValidation(schema, elem.Type)
			if resolvedType == nil {
				return nil, fmt.Errorf("cannot resolve element type for '%s'", formatNodeTest(test))
			}
			return resolvedType, nil
		}
		// even if name doesn't match, if this element has complex type, search its content
		if elem.Type != nil {
			if resolvedType := resolveTypeForValidation(schema, elem.Type); resolvedType != nil {
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
		var unresolvedErr error
		for _, childParticle := range p.Particles {
			if typ, err := findElementInParticleDescendant(schema, childParticle, test, visited); err == nil {
				return typ, nil
			} else if errors.Is(err, ErrXPathUnresolvable) && unresolvedErr == nil {
				unresolvedErr = err
			}
		}
		if unresolvedErr != nil {
			return nil, unresolvedErr
		}
		return nil, fmt.Errorf("element '%s' not found in model group", formatNodeTest(test))
	case *types.AnyElement:
		// wildcard - cannot determine specific element type
		return nil, fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}

	return nil, fmt.Errorf("element '%s' not found in particle", formatNodeTest(test))
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
			if resolvedType := resolveTypeForValidation(schema, elem.Type); resolvedType != nil {
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

// findAttributeType finds the type of an attribute in an element's type.
func findAttributeType(schema *parser.Schema, elementDecl *types.ElementDecl, test xpath.NodeTest) (types.Type, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
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
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
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
		elem := resolveElementReference(schema, p)
		if attrType, err := findAttributeType(schema, elem, test); err == nil {
			return attrType, nil
		}
		if elem.Type != nil {
			if resolvedType := resolveTypeForValidation(schema, elem.Type); resolvedType != nil {
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
	case *types.AnyElement:
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
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
		return nil, fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
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
