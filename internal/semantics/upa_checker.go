package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

type upaChecker struct {
	schema       *parser.Schema
	substMembers map[model.QName][]model.QName
	substAllowed map[substKey]bool
}

type substKey struct {
	head   model.QName
	member model.QName
}

func newUPAChecker(schema *parser.Schema) *upaChecker {
	return &upaChecker{
		schema:       schema,
		substMembers: make(map[model.QName][]model.QName),
		substAllowed: make(map[substKey]bool),
	}
}

func (c *upaChecker) positionsOverlap(left, right Position) bool {
	if left.Kind == PositionWildcard && right.Kind == PositionWildcard && left.Wildcard == right.Wildcard {
		return false
	}
	switch {
	case left.Kind == PositionElement && right.Kind == PositionElement:
		return c.elementPositionsOverlap(left, right)
	case left.Kind == PositionElement && right.Kind == PositionWildcard:
		return c.elementWildcardOverlap(left, right)
	case left.Kind == PositionWildcard && right.Kind == PositionElement:
		return c.elementWildcardOverlap(right, left)
	case left.Kind == PositionWildcard && right.Kind == PositionWildcard:
		return wildcardsOverlap(left.Wildcard, right.Wildcard)
	default:
		return false
	}
}

func (c *upaChecker) elementPositionsOverlap(left, right Position) bool {
	if left.Element == nil || right.Element == nil {
		return false
	}
	if left.Element.Name == right.Element.Name {
		return true
	}
	if c == nil || c.schema == nil {
		return false
	}
	if left.AllowsSubst && c.isSubstitutable(left.Element.Name, right.Element.Name) {
		return true
	}
	if right.AllowsSubst && c.isSubstitutable(right.Element.Name, left.Element.Name) {
		return true
	}
	return false
}

func (c *upaChecker) elementWildcardOverlap(elem, wildcard Position) bool {
	if elem.Element == nil || wildcard.Wildcard == nil {
		return false
	}
	if model.AllowsNamespace(
		wildcard.Wildcard.Namespace,
		wildcard.Wildcard.NamespaceList,
		wildcard.Wildcard.TargetNamespace,
		elem.Element.Name.Namespace,
	) {
		return true
	}
	if c == nil || c.schema == nil || !elem.AllowsSubst {
		return false
	}
	for _, member := range c.substitutionMembers(elem.Element.Name) {
		if !c.isSubstitutable(elem.Element.Name, member) {
			continue
		}
		if model.AllowsNamespace(
			wildcard.Wildcard.Namespace,
			wildcard.Wildcard.NamespaceList,
			wildcard.Wildcard.TargetNamespace,
			member.Namespace,
		) {
			return true
		}
	}
	return false
}

func wildcardsOverlap(left, right *model.AnyElement) bool {
	if left == nil || right == nil {
		return false
	}
	return model.IntersectAnyElement(left, right) != nil
}

func (c *upaChecker) substitutionMembers(head model.QName) []model.QName {
	if c == nil || c.schema == nil {
		return nil
	}
	if cached, ok := c.substMembers[head]; ok {
		return cached
	}
	visited := make(map[model.QName]bool)
	queue := []model.QName{head}
	visited[head] = true
	var out []model.QName
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, member := range c.schema.SubstitutionGroups[current] {
			if visited[member] {
				continue
			}
			visited[member] = true
			out = append(out, member)
			queue = append(queue, member)
		}
	}
	c.substMembers[head] = out
	return out
}

func (c *upaChecker) isSubstitutable(head, member model.QName) bool {
	if c == nil || c.schema == nil {
		return isSubstitutableElement(nil, head, member)
	}
	key := substKey{head: head, member: member}
	if allowed, ok := c.substAllowed[key]; ok {
		return allowed
	}
	allowed := isSubstitutableElement(c.schema, head, member)
	c.substAllowed[key] = allowed
	return allowed
}

func isSubstitutableElement(schema *parser.Schema, head, member model.QName) bool {
	if schema == nil || head == member {
		return true
	}
	headDecl := schema.ElementDecls[head]
	if headDecl == nil {
		return false
	}
	if headDecl.Block.Has(model.DerivationSubstitution) {
		return false
	}
	if !isSubstitutionGroupMember(schema, head, member) {
		return false
	}
	memberDecl := schema.ElementDecls[member]
	if memberDecl == nil {
		return false
	}
	headType := typeresolve.ResolveTypeReference(schema, headDecl.Type, typeresolve.TypeReferenceAllowMissing)
	memberType := typeresolve.ResolveTypeReference(schema, memberDecl.Type, typeresolve.TypeReferenceAllowMissing)
	if headType == nil || memberType == nil {
		return true
	}
	combinedBlock := headDecl.Block
	if headCT, ok := headType.(*model.ComplexType); ok {
		combinedBlock = combinedBlock.Add(model.DerivationMethod(headCT.Block))
	}
	if isDerivationBlocked(memberType, headType, combinedBlock) {
		return false
	}
	return true
}

func isSubstitutionGroupMember(schema *parser.Schema, head, member model.QName) bool {
	if schema == nil {
		return false
	}
	visited := make(map[model.QName]bool)
	var walk func(model.QName) bool
	walk = func(current model.QName) bool {
		if visited[current] {
			return false
		}
		visited[current] = true
		for _, sub := range schema.SubstitutionGroups[current] {
			if sub == member {
				return true
			}
			if walk(sub) {
				return true
			}
		}
		return false
	}
	return walk(head)
}

func isDerivationBlocked(memberType, headType model.Type, block model.DerivationSet) bool {
	if memberType == nil || headType == nil || block == 0 {
		return false
	}
	current := memberType
	for current != nil && current != headType {
		method := derivationMethodForType(current)
		if method != 0 && block.Has(method) {
			return true
		}
		derived, ok := model.AsDerivedType(current)
		if !ok {
			return false
		}
		current = derived.ResolvedBaseType()
	}
	return false
}

func derivationMethodForType(typ model.Type) model.DerivationMethod {
	switch typed := typ.(type) {
	case *model.ComplexType:
		return typed.DerivationMethod
	case *model.SimpleType:
		if typed.List != nil || typed.Variety() == model.ListVariety {
			return model.DerivationList
		}
		if typed.Union != nil || typed.Variety() == model.UnionVariety {
			return model.DerivationUnion
		}
		if typed.Restriction != nil || typed.ResolvedBase != nil {
			return model.DerivationRestriction
		}
	case *model.BuiltinType:
		return model.DerivationRestriction
	}
	return 0
}
