package schemair

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/xsdpath"
)

var (
	errIdentityXPathUnresolvable          = errors.New("xpath cannot be resolved statically")
	errIdentityFieldSelectsNillable       = errors.New("field selects nillable element")
	errIdentityFieldSelectsComplexContent = errors.New("field selects element with complex content")
	errIdentityFieldIncompatibleTypes     = errors.New("field xpath resolves to incompatible element types")
)

type identityFieldResolution struct {
	TypeDecl TypeRef
}

func (r *docResolver) resolveIdentityField(
	element ElementID,
	kind IdentityKind,
	selectorXPath string,
	selectorExpr xsdpath.Expression,
	fieldXPath string,
	fieldExpr xsdpath.Expression,
) (identityFieldResolution, error) {
	selectorElems, err := r.resolveIdentitySelectorElements(element, selectorXPath, selectorExpr)
	if err != nil {
		return identityFieldResolution{}, nil
	}
	fieldTypes, unresolved, nillable, err := r.resolveIdentityFieldTypes(selectorElems, fieldXPath, fieldExpr)
	if err != nil {
		return identityFieldResolution{}, nil
	}
	if kind == IdentityKey && nillable {
		return identityFieldResolution{}, fmt.Errorf("schema ir: field xpath %q selects nillable element", fieldXPath)
	}
	typeRef, err := r.combineIdentityFieldTypes(fieldXPath, fieldTypes)
	if err != nil {
		return identityFieldResolution{}, nil
	}
	if unresolved {
		return identityFieldResolution{}, nil
	}
	return identityFieldResolution{TypeDecl: typeRef}, nil
}

func (r *docResolver) resolveIdentitySelectorElements(element ElementID, xpath string, expr xsdpath.Expression) ([]ElementID, error) {
	var out []ElementID
	unresolved := false
	for branch, path := range expr.Paths {
		if path.Attribute != nil {
			return nil, fmt.Errorf("selector xpath cannot select attributes")
		}
		id, err := r.resolveIdentityPathElement(element, path.Steps)
		if err != nil {
			if errors.Is(err, errIdentityXPathUnresolvable) {
				unresolved = true
				continue
			}
			return nil, fmt.Errorf("branch %d: %w", branch+1, err)
		}
		out = append(out, id)
	}
	if len(out) == 0 {
		if unresolved {
			return nil, fmt.Errorf("%w: selector xpath %q", errIdentityXPathUnresolvable, xpath)
		}
		return nil, fmt.Errorf("cannot resolve selector element")
	}
	return uniqueElementIDs(out), nil
}

func (r *docResolver) resolveIdentityFieldTypes(selectorElems []ElementID, _ string, expr xsdpath.Expression) ([]TypeRef, bool, bool, error) {
	var out []TypeRef
	unresolved := false
	nillable := false
	for _, elemID := range selectorElems {
		for _, path := range expr.Paths {
			resolved, err := r.resolveIdentityFieldPathType(elemID, path)
			if err == nil {
				out = append(out, resolved.TypeDecl)
				nillable = nillable || resolved.Nillable
				continue
			}
			if errors.Is(err, errIdentityFieldSelectsNillable) {
				out = append(out, resolved.TypeDecl)
				nillable = true
				continue
			}
			unresolved = true
		}
	}
	if len(out) == 0 && unresolved {
		return nil, true, nillable, nil
	}
	return out, unresolved, nillable, nil
}

type identityFieldPathType struct {
	TypeDecl TypeRef
	Nillable bool
}

func (r *docResolver) resolveIdentityFieldPathType(element ElementID, path xsdpath.Path) (identityFieldPathType, error) {
	if path.Attribute != nil && identityPathIsDescendantAttributeSearch(path.Steps) {
		ref, err := r.findIdentityAttributeTypeDescendant(element, *path.Attribute)
		if err != nil {
			return identityFieldPathType{}, fmt.Errorf("resolve attribute field %s: %w", formatIdentityNodeTest(*path.Attribute), err)
		}
		return identityFieldPathType{TypeDecl: ref}, nil
	}
	selected, err := r.resolveIdentityPathElement(element, path.Steps)
	if err != nil {
		return identityFieldPathType{}, fmt.Errorf("resolve field path: %w", err)
	}
	if path.Attribute != nil {
		ref, err := r.findIdentityAttributeType(selected, *path.Attribute)
		if err != nil {
			return identityFieldPathType{}, fmt.Errorf("resolve attribute field %s: %w", formatIdentityNodeTest(*path.Attribute), err)
		}
		return identityFieldPathType{TypeDecl: ref}, nil
	}
	elem, ok := r.emittedElement(selected)
	if !ok {
		return identityFieldPathType{}, fmt.Errorf("cannot resolve element declaration")
	}
	ref, err := r.identityElementValueType(elem)
	out := identityFieldPathType{TypeDecl: ref, Nillable: elem.Nillable}
	if elem.Nillable {
		return out, errIdentityFieldSelectsNillable
	}
	if err != nil {
		return identityFieldPathType{}, err
	}
	return out, nil
}

func (r *docResolver) resolveIdentityPathElement(start ElementID, steps []xsdpath.Step) (ElementID, error) {
	current := start
	descendantNext := false
	for _, step := range steps {
		switch step.Axis {
		case xsdpath.AxisDescendantOrSelf:
			if !identityWildcardTest(step.Test) {
				return 0, fmt.Errorf("xpath uses disallowed axis")
			}
			if descendantNext {
				return 0, fmt.Errorf("xpath step is missing a node test")
			}
			descendantNext = true
			continue
		case xsdpath.AxisSelf:
			if descendantNext {
				if identityWildcardTest(step.Test) {
					return 0, fmt.Errorf("%w: descendant self step", errIdentityXPathUnresolvable)
				}
				return 0, fmt.Errorf("xpath step is missing a node test")
			}
			if !identityWildcardTest(step.Test) {
				elem, ok := r.emittedElement(current)
				if !ok || !identityNodeTestMatchesName(step.Test, elem.Name) {
					return 0, fmt.Errorf("xpath self step does not match current element")
				}
			}
			continue
		case xsdpath.AxisChild:
		default:
			return 0, fmt.Errorf("xpath uses disallowed axis")
		}
		if identityWildcardTest(step.Test) {
			return 0, fmt.Errorf("%w: wildcard element", errIdentityXPathUnresolvable)
		}
		var err error
		if descendantNext {
			current, err = r.findIdentityDescendantElement(current, step.Test)
			descendantNext = false
		} else {
			current, err = r.findIdentityChildElement(current, step.Test)
		}
		if err != nil {
			return 0, err
		}
	}
	if descendantNext {
		return 0, fmt.Errorf("xpath step is missing a node test")
	}
	if current == 0 {
		return 0, fmt.Errorf("cannot resolve element declaration")
	}
	return current, nil
}

func (r *docResolver) findIdentityChildElement(element ElementID, test xsdpath.NodeTest) (ElementID, error) {
	plan, ok, err := r.identityElementComplexPlan(element)
	if err != nil {
		return 0, err
	}
	if !ok || plan.Particle == 0 {
		return 0, fmt.Errorf("element %s not found in content model", formatIdentityNodeTest(test))
	}
	found, unresolved, err := r.findIdentityElementInParticle(plan.Particle, test, identitySearchDirect, nil)
	if err != nil {
		return 0, err
	}
	if found != 0 {
		return found, nil
	}
	if unresolved {
		return 0, fmt.Errorf("%w: wildcard element", errIdentityXPathUnresolvable)
	}
	return 0, fmt.Errorf("element %s not found in content model", formatIdentityNodeTest(test))
}

func (r *docResolver) findIdentityDescendantElement(element ElementID, test xsdpath.NodeTest) (ElementID, error) {
	plan, ok, err := r.identityElementComplexPlan(element)
	if err != nil {
		return 0, err
	}
	if !ok || plan.Particle == 0 {
		return 0, fmt.Errorf("element %s not found in content model", formatIdentityNodeTest(test))
	}
	visited := map[TypeID]bool{plan.TypeDecl: true}
	found, unresolved, err := r.findIdentityElementInParticle(plan.Particle, test, identitySearchDescendant, visited)
	if err != nil {
		return 0, err
	}
	if found != 0 {
		return found, nil
	}
	if unresolved {
		return 0, fmt.Errorf("%w: wildcard element", errIdentityXPathUnresolvable)
	}
	return 0, fmt.Errorf("element %s not found in content model", formatIdentityNodeTest(test))
}

type identitySearchMode uint8

const (
	identitySearchDirect identitySearchMode = iota
	identitySearchDescendant
)

func (r *docResolver) findIdentityElementInParticle(id ParticleID, test xsdpath.NodeTest, mode identitySearchMode, visited map[TypeID]bool) (ElementID, bool, error) {
	particle, ok, err := r.particle(id)
	if err != nil || !ok {
		return 0, false, err
	}
	switch particle.Kind {
	case ParticleElement:
		elem, ok := r.emittedElement(particle.Element)
		if ok && identityNodeTestMatchesName(test, elem.Name) {
			return particle.Element, false, nil
		}
		if mode != identitySearchDescendant {
			return 0, false, nil
		}
		found, unresolved, err := r.findIdentityDescendantElementContent(particle.Element, test, visited)
		if err != nil || found != 0 {
			return found, unresolved, err
		}
		return 0, unresolved, nil
	case ParticleGroup:
		unresolved := false
		for _, child := range particle.Children {
			found, childUnresolved, err := r.findIdentityElementInParticle(child, test, mode, visited)
			if err != nil {
				return 0, false, err
			}
			if found != 0 {
				return found, false, nil
			}
			unresolved = unresolved || childUnresolved
		}
		return 0, unresolved, nil
	case ParticleWildcard:
		return 0, true, nil
	default:
		return 0, false, nil
	}
}

func (r *docResolver) findIdentityDescendantElementContent(element ElementID, test xsdpath.NodeTest, visited map[TypeID]bool) (ElementID, bool, error) {
	plan, ok, err := r.identityElementComplexPlan(element)
	if err != nil || !ok || plan.Particle == 0 {
		return 0, false, err
	}
	if visited == nil {
		visited = make(map[TypeID]bool)
	}
	if visited[plan.TypeDecl] {
		return 0, false, nil
	}
	visited[plan.TypeDecl] = true
	return r.findIdentityElementInParticle(plan.Particle, test, identitySearchDescendant, visited)
}

func (r *docResolver) findIdentityAttributeType(element ElementID, test xsdpath.NodeTest) (TypeRef, error) {
	if identityWildcardTest(test) {
		return TypeRef{}, fmt.Errorf("%w: wildcard attribute", errIdentityXPathUnresolvable)
	}
	plan, ok, err := r.identityElementComplexPlan(element)
	if err != nil {
		return TypeRef{}, err
	}
	if !ok {
		return TypeRef{}, fmt.Errorf("attribute %s not found in element type", formatIdentityNodeTest(test))
	}
	for _, id := range plan.Attrs {
		if id == 0 || int(id) > len(r.out.AttributeUses) {
			continue
		}
		use := r.out.AttributeUses[id-1]
		if use.Use == AttributeProhibited {
			continue
		}
		if identityNodeTestMatchesName(test, use.Name) {
			return use.TypeDecl, nil
		}
	}
	if plan.AnyAttr != 0 {
		return TypeRef{}, fmt.Errorf("%w: anyAttribute", errIdentityXPathUnresolvable)
	}
	return TypeRef{}, fmt.Errorf("attribute %s not found in element type", formatIdentityNodeTest(test))
}

func (r *docResolver) findIdentityAttributeTypeDescendant(element ElementID, test xsdpath.NodeTest) (TypeRef, error) {
	if ref, err := r.findIdentityAttributeType(element, test); err == nil {
		return ref, nil
	}
	plan, ok, err := r.identityElementComplexPlan(element)
	if err != nil || !ok || plan.Particle == 0 {
		return TypeRef{}, err
	}
	found, unresolved, err := r.findIdentityAttributeInParticle(plan.Particle, test, make(map[TypeID]bool))
	if err != nil {
		return TypeRef{}, err
	}
	if !isZeroTypeRef(found) {
		return found, nil
	}
	if unresolved {
		return TypeRef{}, fmt.Errorf("%w: wildcard attribute", errIdentityXPathUnresolvable)
	}
	return TypeRef{}, fmt.Errorf("attribute %s not found in content model", formatIdentityNodeTest(test))
}

func (r *docResolver) findIdentityAttributeInParticle(id ParticleID, test xsdpath.NodeTest, visited map[TypeID]bool) (TypeRef, bool, error) {
	particle, ok, err := r.particle(id)
	if err != nil || !ok {
		return TypeRef{}, false, err
	}
	switch particle.Kind {
	case ParticleElement:
		if ref, err := r.findIdentityAttributeType(particle.Element, test); err == nil {
			return ref, false, nil
		} else if errors.Is(err, errIdentityXPathUnresolvable) {
			return TypeRef{}, true, nil
		}
		plan, ok, err := r.identityElementComplexPlan(particle.Element)
		if err != nil || !ok || plan.Particle == 0 {
			return TypeRef{}, false, err
		}
		if visited[plan.TypeDecl] {
			return TypeRef{}, false, nil
		}
		visited[plan.TypeDecl] = true
		return r.findIdentityAttributeInParticle(plan.Particle, test, visited)
	case ParticleGroup:
		unresolved := false
		for _, child := range particle.Children {
			ref, childUnresolved, err := r.findIdentityAttributeInParticle(child, test, visited)
			if err != nil {
				return TypeRef{}, false, err
			}
			if !isZeroTypeRef(ref) {
				return ref, false, nil
			}
			unresolved = unresolved || childUnresolved
		}
		return TypeRef{}, unresolved, nil
	case ParticleWildcard:
		return TypeRef{}, true, nil
	default:
		return TypeRef{}, false, nil
	}
}

func (r *docResolver) identityElementComplexPlan(element ElementID) (ComplexTypePlan, bool, error) {
	elem, ok := r.emittedElement(element)
	if !ok {
		return ComplexTypePlan{}, false, fmt.Errorf("element %d not emitted", element)
	}
	if elem.TypeDecl.Builtin {
		if elem.TypeDecl.Name.Local == "anyType" {
			return ComplexTypePlan{}, false, fmt.Errorf("%w: anyType content", errIdentityXPathUnresolvable)
		}
		return ComplexTypePlan{}, false, nil
	}
	info, ok, err := r.typeInfoForRef(elem.TypeDecl)
	if err != nil || !ok {
		return ComplexTypePlan{}, false, err
	}
	if info.Kind != TypeComplex {
		return ComplexTypePlan{}, false, nil
	}
	plan, ok := r.complexPlan(elem.TypeDecl.ID)
	if ok {
		return plan, true, nil
	}
	if decl := r.complexDecls[r.complexByID[elem.TypeDecl.ID]]; decl != nil {
		if _, err := r.ensureComplexType(decl, !decl.Name.IsZero()); err != nil {
			return ComplexTypePlan{}, false, err
		}
	}
	plan, ok = r.complexPlan(elem.TypeDecl.ID)
	return plan, ok, nil
}

func (r *docResolver) identityElementValueType(elem Element) (TypeRef, error) {
	if elem.TypeDecl.Builtin {
		if elem.TypeDecl.Name.Local == "anyType" {
			return TypeRef{}, errIdentityFieldSelectsComplexContent
		}
		return elem.TypeDecl, nil
	}
	info, ok, err := r.typeInfoForRef(elem.TypeDecl)
	if err != nil || !ok {
		return TypeRef{}, err
	}
	if info.Kind != TypeComplex {
		return elem.TypeDecl, nil
	}
	plan, ok, err := r.identityElementComplexPlan(elem.ID)
	if err != nil || !ok {
		return TypeRef{}, errIdentityFieldSelectsComplexContent
	}
	if !isZeroTypeRef(plan.TextType) {
		return plan.TextType, nil
	}
	if !isZeroSimpleTypeSpec(plan.TextSpec) {
		return TypeRef{}, nil
	}
	return TypeRef{}, errIdentityFieldSelectsComplexContent
}

func (r *docResolver) combineIdentityFieldTypes(xpath string, values []TypeRef) (TypeRef, error) {
	unique := uniqueTypeRefs(values)
	if len(unique) == 0 {
		return TypeRef{}, fmt.Errorf("field xpath %q resolves to no types", xpath)
	}
	if len(unique) == 1 {
		return unique[0], nil
	}
	for i := range unique {
		for j := i + 1; j < len(unique); j++ {
			ok, err := r.identityFieldTypesCompatible(unique[i], unique[j])
			if err != nil {
				return TypeRef{}, err
			}
			if !ok {
				return TypeRef{}, fmt.Errorf("%w: field xpath %q selects incompatible types %s and %s",
					errIdentityFieldIncompatibleTypes, xpath, formatName(unique[i].Name), formatName(unique[j].Name))
			}
		}
	}
	return unique[0], nil
}

func uniqueTypeRefs(values []TypeRef) []TypeRef {
	seen := make(map[TypeRef]struct{}, len(values))
	out := make([]TypeRef, 0, len(values))
	for _, value := range values {
		if isZeroTypeRef(value) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (r *docResolver) identityFieldTypesCompatible(a, b TypeRef) (bool, error) {
	if isZeroTypeRef(a) || isZeroTypeRef(b) {
		return false, nil
	}
	if sameTypeRef(a, b) {
		return true, nil
	}
	if r.typeRefDerivesFrom(a, b, make(map[TypeRef]bool)) || r.typeRefDerivesFrom(b, a, make(map[TypeRef]bool)) {
		return true, nil
	}
	aSpec, aOK := r.specForRef(a)
	bSpec, bOK := r.specForRef(b)
	if aOK && bOK && aSpec.Primitive != "" && aSpec.Primitive == bSpec.Primitive {
		return true, nil
	}
	return false, nil
}

func identityPathIsDescendantAttributeSearch(steps []xsdpath.Step) bool {
	if len(steps) == 0 {
		return false
	}
	sawDescendant := false
	for _, step := range steps {
		switch step.Axis {
		case xsdpath.AxisDescendantOrSelf:
			if !identityWildcardTest(step.Test) {
				return false
			}
			sawDescendant = true
		case xsdpath.AxisSelf:
			if !identityWildcardTest(step.Test) {
				return false
			}
		default:
			return false
		}
	}
	return sawDescendant
}

func uniqueElementIDs(values []ElementID) []ElementID {
	seen := make(map[ElementID]struct{}, len(values))
	out := make([]ElementID, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func identityWildcardTest(test xsdpath.NodeTest) bool {
	return test.Any || test.Local == "*"
}

func identityNodeTestMatchesName(test xsdpath.NodeTest, name Name) bool {
	test = xsdpath.CanonicalizeNodeTest(test)
	if test.Any {
		return true
	}
	if test.Local != "*" && name.Local != test.Local {
		return false
	}
	if test.NamespaceSpecified && string(test.Namespace) != name.Namespace {
		return false
	}
	return true
}

func formatIdentityNodeTest(test xsdpath.NodeTest) string {
	if identityWildcardTest(test) {
		return "*"
	}
	if !test.NamespaceSpecified || test.Namespace == "" {
		return test.Local
	}
	return "{" + string(test.Namespace) + "}" + test.Local
}
