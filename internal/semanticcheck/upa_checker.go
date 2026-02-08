package semanticcheck

import (
	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type upaChecker struct {
	schema       *parser.Schema
	substMembers map[types.QName][]types.QName
	substAllowed map[substKey]bool
}

type substKey struct {
	head   types.QName
	member types.QName
}

func newUPAChecker(schema *parser.Schema) *upaChecker {
	return &upaChecker{
		schema:       schema,
		substMembers: make(map[types.QName][]types.QName),
		substAllowed: make(map[substKey]bool),
	}
}

func (c *upaChecker) positionsOverlap(left, right models.Position) bool {
	if left.Kind == models.PositionWildcard && right.Kind == models.PositionWildcard && left.Wildcard == right.Wildcard {
		return false
	}
	switch {
	case left.Kind == models.PositionElement && right.Kind == models.PositionElement:
		return c.elementPositionsOverlap(left, right)
	case left.Kind == models.PositionElement && right.Kind == models.PositionWildcard:
		return c.elementWildcardOverlap(left, right)
	case left.Kind == models.PositionWildcard && right.Kind == models.PositionElement:
		return c.elementWildcardOverlap(right, left)
	case left.Kind == models.PositionWildcard && right.Kind == models.PositionWildcard:
		return wildcardsOverlap(left.Wildcard, right.Wildcard)
	default:
		return false
	}
}

func (c *upaChecker) elementPositionsOverlap(left, right models.Position) bool {
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

func (c *upaChecker) elementWildcardOverlap(elem, wildcard models.Position) bool {
	if elem.Element == nil || wildcard.Wildcard == nil {
		return false
	}
	if wildcardMatchesQName(wildcard.Wildcard, elem.Element.Name) {
		return true
	}
	if c == nil || c.schema == nil || !elem.AllowsSubst {
		return false
	}
	for _, member := range c.substitutionMembers(elem.Element.Name) {
		if !c.isSubstitutable(elem.Element.Name, member) {
			continue
		}
		if wildcardMatchesQName(wildcard.Wildcard, member) {
			return true
		}
	}
	return false
}

func wildcardsOverlap(left, right *types.AnyElement) bool {
	if left == nil || right == nil {
		return false
	}
	return types.IntersectAnyElement(left, right) != nil
}

func wildcardMatchesQName(wildcard *types.AnyElement, qname types.QName) bool {
	if wildcard == nil {
		return false
	}
	return types.AllowsNamespace(wildcard.Namespace, wildcard.NamespaceList, wildcard.TargetNamespace, qname.Namespace)
}

func (c *upaChecker) substitutionMembers(head types.QName) []types.QName {
	if c == nil || c.schema == nil {
		return nil
	}
	if cached, ok := c.substMembers[head]; ok {
		return cached
	}
	visited := make(map[types.QName]bool)
	queue := []types.QName{head}
	visited[head] = true
	var out []types.QName
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

func (c *upaChecker) isSubstitutable(head, member types.QName) bool {
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
