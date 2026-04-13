package semantics

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ErrFieldSelectsComplexContent indicates a field XPath selects an element with
// complex content, which is invalid per XSD 1.0 identity-constraint rules.
var ErrFieldSelectsComplexContent = errors.New("field selects element with complex content")

// ErrXPathUnresolvable indicates a selector or field XPath cannot be resolved
// statically, such as when wildcard steps are present.
var ErrXPathUnresolvable = errors.New("xpath cannot be resolved statically")

// ErrFieldXPathIncompatibleTypes indicates a field XPath resolves to
// incompatible element types.
var ErrFieldXPathIncompatibleTypes = errors.New("field xpath resolves to incompatible element types")

// ErrFieldXPathUnresolved indicates a field XPath cannot be resolved.
var ErrFieldXPathUnresolved = errors.New("field xpath unresolved")

// ErrFieldSelectsNillable indicates a field XPath selects a nillable element.
var ErrFieldSelectsNillable = errors.New("field selects nillable element")

func isWildcardNodeTest(test runtime.NodeTest) bool {
	return test.Any || test.Local == "*"
}

func nodeTestMatchesQName(test runtime.NodeTest, name model.QName) bool {
	test = runtime.CanonicalizeNodeTest(test)
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

func resolveElementReference(schema *parser.Schema, decl *model.ElementDecl) *model.ElementDecl {
	if decl == nil || !decl.IsReference || schema == nil {
		return decl
	}
	if resolved, ok := schema.ElementDecls[decl.Name]; ok {
		return resolved
	}
	return decl
}

func formatNodeTest(test runtime.NodeTest) string {
	if isWildcardNodeTest(test) {
		return "*"
	}
	if !test.NamespaceSpecified || test.Namespace == "" {
		return test.Local
	}
	return "{" + test.Namespace + "}" + test.Local
}

func fieldTypeName(typ model.Type) string {
	if typ == nil {
		return "<nil>"
	}
	name := typ.Name()
	if !name.IsZero() {
		return name.String()
	}
	return fmt.Sprintf("%T", typ)
}

func fieldTypeKey(typ model.Type) string {
	if typ == nil {
		return ""
	}
	name := typ.Name()
	if !name.IsZero() {
		return name.String()
	}
	return fmt.Sprintf("%T:%p", typ, typ)
}

func uniqueFieldTypes(values []model.Type) []model.Type {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	unique := make([]model.Type, 0, len(values))
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

// FieldTypesCompatible reports whether two field types are compatible for
// identity-constraint matching.
func FieldTypesCompatible(a, b model.Type) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name() == b.Name() {
		return true
	}
	if model.IsDerivedFrom(a, b) || model.IsDerivedFrom(b, a) {
		return true
	}
	primA := a.PrimitiveType()
	primB := b.PrimitiveType()
	if primA != nil && primB != nil && primA.Name() == primB.Name() {
		return true
	}
	return false
}

func combineFieldTypes(fieldXPath string, values []model.Type) (model.Type, error) {
	unique := uniqueFieldTypes(values)
	if len(unique) == 0 {
		return nil, fmt.Errorf("field xpath '%s' resolves to no types", fieldXPath)
	}
	if len(unique) == 1 {
		return unique[0], nil
	}
	for i := range unique {
		for j := i + 1; j < len(unique); j++ {
			if !FieldTypesCompatible(unique[i], unique[j]) {
				return nil, fmt.Errorf("%w: field xpath '%s' selects incompatible types '%s' and '%s'", ErrFieldXPathIncompatibleTypes, fieldXPath, fieldTypeName(unique[i]), fieldTypeName(unique[j]))
			}
		}
	}
	return &model.SimpleType{
		Union:       &model.UnionType{},
		MemberTypes: unique,
	}, nil
}

func isDescendantOnlySteps(steps []runtime.Step) bool {
	if len(steps) == 0 {
		return false
	}
	sawDescendant := false
	for _, step := range steps {
		switch step.Axis {
		case runtime.AxisDescendantOrSelf:
			if !step.Test.Any {
				return false
			}
			sawDescendant = true
		case runtime.AxisSelf:
			if !step.Test.Any {
				return false
			}
		default:
			return false
		}
	}
	return sawDescendant
}

func uniqueElementDecls(decls []*model.ElementDecl) []*model.ElementDecl {
	if len(decls) == 0 {
		return nil
	}
	seen := make(map[model.QName]struct{}, len(decls))
	unique := make([]*model.ElementDecl, 0, len(decls))
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

type fieldPathBranch struct {
	selectorDecl *model.ElementDecl
	path         runtime.Path
	pathIndex    int
}

func hasFieldPathUnion(selectorDecls []*model.ElementDecl, paths []runtime.Path) bool {
	return len(selectorDecls) > 1 || len(paths) > 1
}

func forEachFieldPathBranch(selectorDecls []*model.ElementDecl, paths []runtime.Path, fn func(fieldPathBranch) error) error {
	for _, selectorDecl := range selectorDecls {
		for pathIndex, path := range paths {
			if err := fn(fieldPathBranch{
				selectorDecl: selectorDecl,
				path:         path,
				pathIndex:    pathIndex,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func wrapFieldPathBranchError(fieldXPath string, branch fieldPathBranch, err error) error {
	return wrapXPathBranchError("field", fieldXPath, branch, err)
}

func wrapXPathBranchError(kind, expr string, branch fieldPathBranch, err error) error {
	return fmt.Errorf("resolve %s xpath '%s' branch %d: %w", kind, expr, branch.pathIndex+1, err)
}

// validateSelectorXPath validates that a selector XPath selects element nodes.
func validateSelectorXPath(expr string) error {
	return validateSelectorXPathWithContext(expr, nil)
}

func validateSelectorXPathWithContext(expr string, nsContext map[string]string) error {
	expr = model.TrimXMLWhitespace(expr)
	if expr == "" {
		return fmt.Errorf("selector xpath cannot be empty")
	}
	if strings.Contains(expr, "/text()") || strings.HasSuffix(expr, "text()") {
		return fmt.Errorf("selector xpath cannot select text nodes: %s", expr)
	}
	if strings.Contains(expr, "..") || strings.Contains(expr, "parent::") {
		return fmt.Errorf("selector xpath cannot use parent navigation: %s", expr)
	}
	if strings.Contains(expr, "attribute::") {
		return fmt.Errorf("selector xpath cannot use axis 'attribute::': %s", expr)
	}

	_, err := runtime.Parse(expr, nsContext, runtime.AttributesDisallowed)
	if err == nil {
		return nil
	}
	return normalizeSelectorXPathError(expr, err)
}

// validateFieldXPath performs checks for field XPath expressions.
func validateFieldXPath(expr string) error {
	return validateFieldXPathWithContext(expr, nil)
}

func validateFieldXPathWithContext(expr string, nsContext map[string]string) error {
	expr = model.TrimXMLWhitespace(expr)
	if expr == "" {
		return fmt.Errorf("field xpath cannot be empty")
	}
	_, err := runtime.Parse(expr, nsContext, runtime.AttributesAllowed)
	if err == nil {
		return nil
	}
	return normalizeFieldXPathError(expr, err)
}

func parseXPathExpression(expr string, nsContext map[string]string, policy runtime.AttributePolicy) (runtime.Expression, error) {
	parsed, err := runtime.Parse(expr, nsContext, policy)
	if err != nil {
		return runtime.Expression{}, err
	}
	if len(parsed.Paths) == 0 {
		return runtime.Expression{}, fmt.Errorf("xpath contains no paths")
	}
	return parsed, nil
}

func normalizeSelectorXPathError(expr string, err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "xpath cannot be empty"):
		return fmt.Errorf("selector xpath cannot be empty")
	case strings.Contains(msg, "xpath must be a relative path"):
		return fmt.Errorf("selector xpath must be a relative path: %s", expr)
	case strings.Contains(msg, "xpath cannot select attributes"):
		return fmt.Errorf("selector xpath cannot select attributes: %s", expr)
	case strings.Contains(msg, "xpath cannot use functions or parentheses"):
		return fmt.Errorf("selector xpath cannot use functions or parentheses: %s", expr)
	case strings.Contains(msg, "xpath uses disallowed axis"):
		if axis := disallowedAxisFromError(msg); axis != "" {
			return fmt.Errorf("selector xpath cannot use axis '%s': %s", axis, expr)
		}
		return fmt.Errorf("selector xpath uses disallowed axis: %s", expr)
	}
	return err
}

func normalizeFieldXPathError(expr string, err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "xpath cannot be empty"):
		return fmt.Errorf("field xpath cannot be empty")
	case strings.Contains(msg, "xpath cannot use functions or parentheses"):
		return fmt.Errorf("field xpath cannot use functions or parentheses: %s", expr)
	case strings.Contains(msg, "xpath uses disallowed axis"):
		if axis := disallowedAxisFromError(msg); axis != "" {
			return fmt.Errorf("field xpath cannot use axis '%s': %s", axis, expr)
		}
		return fmt.Errorf("field xpath uses disallowed axis: %s", expr)
	}
	return err
}

func disallowedAxisFromError(msg string) string {
	idx := strings.Index(msg, "'")
	if idx < 0 || idx+1 >= len(msg) {
		return ""
	}
	rest := msg[idx+1:]
	before, _, ok := strings.Cut(rest, "'")
	if !ok {
		return ""
	}
	return before
}

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
				return nil, errors.New("xpath self step does not match current element")
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
	ct, err := resolveComplexTypeForElementSearch(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	visited := map[*model.ComplexType]struct{}{ct: {}}
	decl, err := findElementDeclInContentDescendant(schema, ct.Content(), test, visited)
	if err != nil && ct.Abstract {
		return nil, fmt.Errorf("%w: %w", ErrXPathUnresolvable, err)
	}
	return decl, err
}

func findElementDeclInContentDescendant(schema *parser.Schema, content model.Content, test runtime.NodeTest, visited map[*model.ComplexType]struct{}) (*model.ElementDecl, error) {
	return findElementDeclInContentWithMode(schema, content, test, elementPathSearchDescendant, visited)
}

func findElementDecl(schema *parser.Schema, elementDecl *model.ElementDecl, test runtime.NodeTest) (*model.ElementDecl, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard element", ErrXPathUnresolvable)
	}
	ct, err := resolveComplexTypeForElementSearch(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	return findElementDeclInContent(ct.Content(), test)
}

func findElementDeclInContent(content model.Content, test runtime.NodeTest) (*model.ElementDecl, error) {
	return findElementDeclInContentWithMode(nil, content, test, elementPathSearchDirect, nil)
}

type elementPathSearchMode uint8

const (
	elementPathSearchDirect elementPathSearchMode = iota
	elementPathSearchDescendant
)

func resolveComplexTypeForElementSearch(schema *parser.Schema, elementDecl *model.ElementDecl) (*model.ComplexType, error) {
	return resolveIdentityElementComplexType(schema, elementDecl)
}

func findElementDeclInContentWithMode(schema *parser.Schema, content model.Content, test runtime.NodeTest, mode elementPathSearchMode, visited map[*model.ComplexType]struct{}) (*model.ElementDecl, error) {
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
				decl, err := findElementDeclInParticleWithMode(schema, c.Extension.Particle, test, mode, visited)
				if err == nil {
					return decl, nil
				}
				resultErr = err
			}
			if c.Restriction != nil && c.Restriction.Particle != nil {
				decl, err := findElementDeclInParticleWithMode(schema, c.Restriction.Particle, test, mode, visited)
				if err == nil {
					return decl, nil
				}
				if resultErr == nil {
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

func findElementDeclInParticleWithMode(schema *parser.Schema, particle model.Particle, test runtime.NodeTest, mode elementPathSearchMode, visited map[*model.ComplexType]struct{}) (*model.ElementDecl, error) {
	switch p := particle.(type) {
	case *model.ElementDecl:
		elem := p
		if mode == elementPathSearchDescendant {
			elem = resolveElementReference(schema, p)
		}
		if elem != nil && nodeTestMatchesQName(test, elem.Name) {
			return elem, nil
		}
		if mode == elementPathSearchDescendant && elem != nil && elem.Type != nil {
			if visited == nil {
				visited = make(map[*model.ComplexType]struct{})
			}
			if resolvedType := parser.ResolveTypeReference(schema, elem.Type); resolvedType != nil {
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

// ResolveSelectorElementType resolves the type of the element selected by the
// selector XPath.
func ResolveSelectorElementType(schema *parser.Schema, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) (model.Type, error) {
	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, err
	}

	var elementType model.Type
	for _, decl := range selectorDecls {
		resolved := parser.ResolveTypeReference(schema, decl.Type)
		if resolved == nil {
			return nil, fmt.Errorf("cannot resolve constraint element type")
		}
		if elementType == nil {
			elementType = resolved
			continue
		}
		if !model.ElementTypesCompatible(elementType, resolved) {
			return nil, fmt.Errorf("selector xpath '%s' resolves to multiple element types", selectorXPath)
		}
	}
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve constraint element type")
	}
	return elementType, nil
}

func resolveSelectorElementDecls(schema *parser.Schema, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) ([]*model.ElementDecl, error) {
	if constraintElement == nil {
		return nil, fmt.Errorf("constraint element is nil")
	}
	expr, err := parseXPathExpression(selectorXPath, nsContext, runtime.AttributesDisallowed)
	if err != nil {
		return nil, err
	}
	decls := make([]*model.ElementDecl, 0, len(expr.Paths))
	unresolved := false
	branches := []*model.ElementDecl{constraintElement}
	err = forEachFieldPathBranch(branches, expr.Paths, func(branch fieldPathBranch) error {
		if branch.path.Attribute != nil {
			return fmt.Errorf("selector xpath cannot select attributes: %s", selectorXPath)
		}
		decl, resolveErr := resolvePathElementDecl(schema, branch.selectorDecl, branch.path.Steps)
		if resolveErr != nil {
			if errors.Is(resolveErr, ErrXPathUnresolvable) {
				unresolved = true
				return nil
			}
			return wrapXPathBranchError("selector", selectorXPath, branch, resolveErr)
		}
		decls = append(decls, decl)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		if unresolved {
			return nil, fmt.Errorf("%w: selector xpath '%s'", ErrXPathUnresolvable, selectorXPath)
		}
		return nil, fmt.Errorf("cannot resolve selector element")
	}
	return decls, nil
}

// ResolveFieldElementDecl resolves a field XPath to the selected element
// declaration. It returns nil if the field selects an attribute.
func ResolveFieldElementDecl(schema *parser.Schema, field *model.Field, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) (*model.ElementDecl, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}
	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, runtime.AttributesAllowed)
	if err != nil {
		return nil, err
	}
	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		if errors.Is(err, ErrFieldXPathIncompatibleTypes) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: resolve selector '%s': %w", ErrFieldXPathUnresolved, selectorXPath, err)
	}
	decls, _, err := resolveFieldElementDeclBranches(schema, selectorDecls, field.XPath, fieldExpr.Paths, false)
	if err != nil {
		return nil, err
	}
	unique := uniqueElementDecls(decls)
	if len(unique) != 1 {
		return nil, fmt.Errorf("field xpath '%s' resolves to multiple element declarations", field.XPath)
	}
	return unique[0], nil
}

// ResolveFieldElementDecls resolves all element declarations selected by a
// field XPath.
func ResolveFieldElementDecls(schema *parser.Schema, field *model.Field, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) ([]*model.ElementDecl, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}
	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, runtime.AttributesAllowed)
	if err != nil {
		return nil, err
	}
	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, fmt.Errorf("resolve selector '%s': %w", selectorXPath, err)
	}
	decls, unresolved, err := resolveFieldElementDeclBranches(schema, selectorDecls, field.XPath, fieldExpr.Paths, true)
	if err != nil {
		return nil, err
	}
	unique := uniqueElementDecls(decls)
	if len(unique) == 0 && unresolved {
		return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
	}
	return unique, nil
}

func resolveFieldElementDeclBranches(schema *parser.Schema, selectorDecls []*model.ElementDecl, fieldXPath string, paths []runtime.Path, tolerateUnresolved bool) ([]*model.ElementDecl, bool, error) {
	hasUnion := hasFieldPathUnion(selectorDecls, paths)
	var decls []*model.ElementDecl
	unresolved := false
	err := forEachFieldPathBranch(selectorDecls, paths, func(branch fieldPathBranch) error {
		if branch.path.Attribute != nil {
			if tolerateUnresolved {
				return nil
			}
			return fmt.Errorf("field xpath selects attribute: %s", fieldXPath)
		}
		decl, err := resolvePathElementDecl(schema, branch.selectorDecl, branch.path.Steps)
		if err != nil {
			if tolerateUnresolved && (hasUnion || errors.Is(err, ErrXPathUnresolvable)) {
				unresolved = true
				return nil
			}
			return wrapFieldPathBranchError(fieldXPath, branch, err)
		}
		decls = append(decls, decl)
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return decls, unresolved, nil
}

// ResolveFieldType resolves the type of a field XPath expression.
func ResolveFieldType(schema *parser.Schema, field *model.Field, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) (model.Type, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}
	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, runtime.AttributesAllowed)
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
	fieldHasUnion := len(fieldExpr.Paths) > 1
	selectorHasUnion := len(selectorDecls) > 1
	resolvedTypes, unresolved, nillableFound, err := resolveFieldTypeBranches(schema, selectorDecls, field.XPath, fieldExpr.Paths)
	if err != nil {
		return nil, err
	}
	if len(resolvedTypes) == 0 {
		if unresolved {
			return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
		}
		return nil, fmt.Errorf("field xpath '%s' resolves to no types", field.XPath)
	}
	combined, err := combineFieldTypes(field.XPath, resolvedTypes)
	if err != nil {
		if selectorHasUnion && !fieldHasUnion && errors.Is(err, ErrFieldXPathIncompatibleTypes) {
			return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
		}
		return nil, err
	}
	if nillableFound {
		return combined, fmt.Errorf("%w: field xpath '%s'", ErrFieldSelectsNillable, field.XPath)
	}
	if unresolved {
		return combined, fmt.Errorf("%w: field xpath '%s' contains wildcard branches", ErrXPathUnresolvable, field.XPath)
	}
	return combined, nil
}

func resolveFieldTypeBranches(schema *parser.Schema, selectorDecls []*model.ElementDecl, fieldXPath string, paths []runtime.Path) ([]model.Type, bool, bool, error) {
	hasUnion := hasFieldPathUnion(selectorDecls, paths)
	var resolvedTypes []model.Type
	unresolved := false
	nillableFound := false
	err := forEachFieldPathBranch(selectorDecls, paths, func(branch fieldPathBranch) error {
		typ, pathErr := resolveFieldPathType(schema, branch.selectorDecl, branch.path)
		if pathErr == nil {
			resolvedTypes = append(resolvedTypes, typ)
			return nil
		}
		if errors.Is(pathErr, ErrFieldSelectsNillable) {
			if typ != nil {
				resolvedTypes = append(resolvedTypes, typ)
			}
			nillableFound = true
			if hasUnion {
				return nil
			}
			return wrapFieldPathBranchError(fieldXPath, branch, pathErr)
		}
		if hasUnion {
			unresolved = true
			return nil
		}
		return wrapFieldPathBranchError(fieldXPath, branch, pathErr)
	})
	if err != nil {
		return nil, false, false, err
	}
	return resolvedTypes, unresolved, nillableFound, nil
}

func resolveFieldPathType(schema *parser.Schema, selectedElementDecl *model.ElementDecl, fieldPath runtime.Path) (model.Type, error) {
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
	elementType, err := resolveIdentityElementType(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	if elementDecl.Nillable {
		return elementType, ErrFieldSelectsNillable
	}
	if ct, ok := elementType.(*model.ComplexType); ok {
		if _, ok := ct.Content().(*model.SimpleContent); ok {
			baseType := ct.BaseType()
			if baseType != nil {
				return baseType, nil
			}
		}
		return nil, ErrFieldSelectsComplexContent
	}
	return elementType, nil
}

// ValidateIdentityConstraintResolution validates that identity-constraint
// selectors and fields can be resolved against the schema.
func ValidateIdentityConstraintResolution(sch *parser.Schema, constraint *model.IdentityConstraint, decl *model.ElementDecl) error {
	for i := range constraint.Fields {
		field := &constraint.Fields[i]
		hasUnion := strings.Contains(field.XPath, "|") || strings.Contains(constraint.Selector.XPath, "|")
		resolved, err := ResolveFieldType(sch, field, decl, constraint.Selector.XPath, constraint.NamespaceContext)
		switch {
		case err == nil:
			field.ResolvedType = resolved
		case errors.Is(err, ErrFieldSelectsNillable):
			if resolved != nil {
				field.ResolvedType = resolved
			}
			if constraint.Type == model.KeyConstraint {
				return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
			}
			continue
		case errors.Is(err, ErrFieldSelectsComplexContent):
			continue
		case hasUnion:
			if !errors.Is(err, ErrXPathUnresolvable) && !errors.Is(err, ErrFieldXPathIncompatibleTypes) {
				return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
			}
		default:
			if !errors.Is(err, ErrXPathUnresolvable) && !errors.Is(err, ErrFieldXPathIncompatibleTypes) {
				return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
			}
		}
		if constraint.Type == model.KeyConstraint {
			if hasUnion {
				elemDecls, err := ResolveFieldElementDecls(sch, field, decl, constraint.Selector.XPath, constraint.NamespaceContext)
				if err != nil {
					if errors.Is(err, ErrXPathUnresolvable) {
						continue
					}
					continue
				}
				for _, elemDecl := range elemDecls {
					if elemDecl != nil && elemDecl.Nillable {
						return fmt.Errorf("field %d '%s' selects nillable element '%s'", i+1, field.XPath, elemDecl.Name)
					}
				}
				continue
			}
			elemDecl, err := ResolveFieldElementDecl(sch, field, decl, constraint.Selector.XPath, constraint.NamespaceContext)
			if err == nil && elemDecl != nil && elemDecl.Nillable {
				return fmt.Errorf("field %d '%s' selects nillable element '%s'", i+1, field.XPath, elemDecl.Name)
			}
		}
	}
	return nil
}

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

// CollectConstraintElementsFromContent returns non-reference elements with
// identity constraints found in the given content tree.
func CollectConstraintElementsFromContent(content model.Content) []*model.ElementDecl {
	state := newIdentityTraversalState()
	out := make([]*model.ElementDecl, 0)
	walkIdentityContent(content, state, func(elem *model.ElementDecl) {
		if elem == nil || elem.IsReference || len(elem.Constraints) == 0 {
			return
		}
		out = append(out, elem)
	})
	return out
}

// CollectAllIdentityConstraints returns all identity constraints in deterministic
// schema traversal order.
func CollectAllIdentityConstraints(sch *parser.Schema) []*model.IdentityConstraint {
	if sch == nil {
		return nil
	}
	var all []*model.IdentityConstraint
	state := newIdentityTraversalState()
	collectConstraints := func(elem *model.ElementDecl) {
		if elem == nil || len(elem.Constraints) == 0 {
			return
		}
		all = append(all, elem.Constraints...)
	}
	collectFromContent := func(content model.Content) {
		walkIdentityContent(content, state, collectConstraints)
	}

	for _, name := range model.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[name]
		all = append(all, decl.Constraints...)
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}
	for _, name := range model.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[name]
		if ct, ok := typ.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}
	for _, name := range model.SortedMapKeys(sch.Groups) {
		group := sch.Groups[name]
		walkIdentityParticles(group.Particles, state, collectConstraints)
	}
	return all
}

// CollectLocalConstraintElements returns local elements with identity
// constraints in deterministic order.
func CollectLocalConstraintElements(sch *parser.Schema) []*model.ElementDecl {
	if sch == nil {
		return nil
	}
	seen := make(map[*model.ElementDecl]bool)
	out := make([]*model.ElementDecl, 0)
	collect := func(content model.Content) {
		for _, elem := range CollectConstraintElementsFromContent(content) {
			if elem == nil || elem.IsReference || len(elem.Constraints) == 0 {
				continue
			}
			if seen[elem] {
				continue
			}
			seen[elem] = true
			out = append(out, elem)
		}
	}
	for _, name := range model.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[name]
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			collect(ct.Content())
		}
	}
	for _, name := range model.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[name]
		if ct, ok := typ.(*model.ComplexType); ok {
			collect(ct.Content())
		}
	}
	return out
}

// ValidateKeyrefConstraints validates keyref constraints against all known
// identity constraints.
func ValidateKeyrefConstraints(contextQName model.QName, constraints, allConstraints []*model.IdentityConstraint) []error {
	var errs []error
	for _, constraint := range constraints {
		if constraint.Type != model.KeyRefConstraint {
			continue
		}
		refQName := constraint.ReferQName
		if refQName.IsZero() {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' is missing refer attribute", contextQName, constraint.Name))
			continue
		}
		if refQName.Namespace != constraint.TargetNamespace {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' refers to '%s' in namespace '%s', which does not match target namespace '%s'", contextQName, constraint.Name, refQName.Local, refQName.Namespace, constraint.TargetNamespace))
			continue
		}
		var referencedConstraint *model.IdentityConstraint
		for _, other := range allConstraints {
			if other.Name == refQName.Local && other.TargetNamespace == refQName.Namespace {
				if other.Type == model.KeyConstraint || other.Type == model.UniqueConstraint {
					referencedConstraint = other
					break
				}
			}
		}
		if referencedConstraint == nil {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' references non-existent key/unique constraint '%s'", contextQName, constraint.Name, refQName.String()))
			continue
		}
		if len(constraint.Fields) != len(referencedConstraint.Fields) {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' has %d fields but referenced constraint '%s' has %d fields", contextQName, constraint.Name, len(constraint.Fields), refQName.String(), len(referencedConstraint.Fields)))
			continue
		}
		for i := 0; i < len(constraint.Fields); i++ {
			keyrefField := constraint.Fields[i]
			refField := referencedConstraint.Fields[i]
			if keyrefField.ResolvedType != nil && refField.ResolvedType != nil && !FieldTypesCompatible(keyrefField.ResolvedType, refField.ResolvedType) {
				errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' field %d type '%s' is not compatible with referenced constraint '%s' field %d type '%s'", contextQName, constraint.Name, i+1, keyrefField.ResolvedType.Name(), refQName.String(), i+1, refField.ResolvedType.Name()))
			}
		}
	}
	return errs
}

// ValidateIdentityConstraintUniqueness reports duplicate identity constraint
// names within the same target namespace.
func ValidateIdentityConstraintUniqueness(allConstraints []*model.IdentityConstraint) []error {
	var errs []error
	type constraintKey struct {
		name      string
		namespace model.NamespaceURI
	}
	constraintsByKey := make(map[constraintKey][]*model.IdentityConstraint)
	for _, constraint := range allConstraints {
		key := constraintKey{name: constraint.Name, namespace: constraint.TargetNamespace}
		constraintsByKey[key] = append(constraintsByKey[key], constraint)
	}
	for key, constraints := range constraintsByKey {
		if len(constraints) > 1 {
			errs = append(errs, fmt.Errorf("identity constraint name '%s' is not unique within target namespace '%s' (%d definitions)", key.name, key.namespace, len(constraints)))
		}
	}
	return errs
}
