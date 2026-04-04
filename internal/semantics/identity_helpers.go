package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/xpath"
)

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

func isWildcardNodeTest(test xpath.NodeTest) bool {
	return test.Any || test.Local == "*"
}

func nodeTestMatchesQName(test xpath.NodeTest, name model.QName) bool {
	test = xpath.CanonicalizeNodeTest(test)
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

func formatNodeTest(test xpath.NodeTest) string {
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
	path         xpath.Path
	pathIndex    int
}

func hasFieldPathUnion(selectorDecls []*model.ElementDecl, paths []xpath.Path) bool {
	return len(selectorDecls) > 1 || len(paths) > 1
}

func forEachFieldPathBranch(selectorDecls []*model.ElementDecl, paths []xpath.Path, fn func(fieldPathBranch) error) error {
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

	for _, name := range qname.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[name]
		all = append(all, decl.Constraints...)
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}
	for _, name := range qname.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[name]
		if ct, ok := typ.(*model.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}
	for _, name := range qname.SortedMapKeys(sch.Groups) {
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
	for _, name := range qname.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[name]
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			collect(ct.Content())
		}
	}
	for _, name := range qname.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[name]
		if ct, ok := typ.(*model.ComplexType); ok {
			collect(ct.Content())
		}
	}
	return out
}
