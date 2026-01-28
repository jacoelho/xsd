package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/models"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateUPA validates Unique Particle Attribution for a content model.
// UPA requires that no element can be matched by more than one particle.
func ValidateUPA(schema *parser.Schema, content types.Content, _ types.NamespaceURI) error {
	var particle types.Particle
	var baseParticle types.Particle

	switch c := content.(type) {
	case *types.ElementContent:
		particle = c.Particle
	case *types.ComplexContent:
		if c.Extension != nil {
			particle = c.Extension.Particle
			if !c.Extension.Base.IsZero() {
				if baseCT, ok := lookupComplexType(schema, c.Extension.Base); ok {
					if baseEC, ok := baseCT.Content().(*types.ElementContent); ok {
						baseParticle = baseEC.Particle
					}
				}
			}
		}
		if c.Restriction != nil {
			particle = c.Restriction.Particle
		}
	}

	if particle == nil && baseParticle == nil {
		return nil
	}

	if particle != nil {
		expanded, err := expandGroupRefs(schema, particle, make(map[types.QName]bool))
		if err != nil {
			return err
		}
		particle = expanded
		relaxOccursInPlace(particle)
	}
	if baseParticle != nil {
		expanded, err := expandGroupRefs(schema, baseParticle, make(map[types.QName]bool))
		if err != nil {
			return err
		}
		baseParticle = expanded
		relaxOccursInPlace(baseParticle)
	}

	if baseParticle != nil && particle != nil {
		particle = &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: types.OccursFromInt(1),
			MaxOccurs: types.OccursFromInt(1),
			Particles: []types.Particle{baseParticle, particle},
		}
	} else if particle == nil {
		particle = baseParticle
	}

	if particle == nil {
		return nil
	}

	glu, err := models.BuildGlushkov(particle)
	if err != nil {
		return err
	}
	checker := newUPAChecker(schema)
	return models.CheckDeterminism(glu, checker.positionsOverlap)
}

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
	if left.Kind == models.PositionElement && right.Kind == models.PositionElement && left.Element == right.Element {
		return false
	}
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

func relaxOccursInPlace(particle types.Particle) {
	switch typed := particle.(type) {
	case *types.ElementDecl:
		typed.MinOccurs, typed.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
	case *types.AnyElement:
		typed.MinOccurs, typed.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
	case *types.ModelGroup:
		typed.MinOccurs, typed.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		for _, child := range typed.Particles {
			relaxOccursInPlace(child)
		}
	}
}

func relaxOccurs(minOccurs, maxOccurs types.Occurs) (types.Occurs, types.Occurs) {
	if maxOccurs.IsUnbounded() || maxOccurs.GreaterThanInt(1) {
		if minOccurs.IsZero() {
			return types.OccursFromInt(0), types.OccursUnbounded
		}
		return types.OccursFromInt(1), types.OccursUnbounded
	}
	return minOccurs, maxOccurs
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

func expandGroupRefs(schema *parser.Schema, particle types.Particle, stack map[types.QName]bool) (types.Particle, error) {
	switch p := particle.(type) {
	case *types.GroupRef:
		if p == nil {
			return nil, nil
		}
		if stack[p.RefQName] {
			return nil, fmt.Errorf("circular group reference detected for %s", p.RefQName)
		}
		if schema == nil {
			return nil, fmt.Errorf("group ref %s not resolved", p.RefQName)
		}
		groupDef, exists := schema.Groups[p.RefQName]
		if !exists {
			return nil, fmt.Errorf("group '%s' not found", p.RefQName)
		}
		stack[p.RefQName] = true
		defer delete(stack, p.RefQName)

		groupCopy := &types.ModelGroup{
			Kind:      groupDef.Kind,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
			Particles: make([]types.Particle, 0, len(groupDef.Particles)),
		}
		if groupCopy.Kind == types.AllGroup {
			groupCopy.Kind = types.Choice
		}
		for _, child := range groupDef.Particles {
			expanded, err := expandGroupRefs(schema, child, stack)
			if err != nil {
				return nil, err
			}
			groupCopy.Particles = append(groupCopy.Particles, expanded)
		}
		return groupCopy, nil
	case *types.ModelGroup:
		if p == nil {
			return nil, nil
		}
		groupCopy := &types.ModelGroup{
			Kind:      p.Kind,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
			Particles: make([]types.Particle, 0, len(p.Particles)),
		}
		if groupCopy.Kind == types.AllGroup {
			groupCopy.Kind = types.Choice
		}
		for _, child := range p.Particles {
			expanded, err := expandGroupRefs(schema, child, stack)
			if err != nil {
				return nil, err
			}
			groupCopy.Particles = append(groupCopy.Particles, expanded)
		}
		return groupCopy, nil
	case *types.ElementDecl:
		if p == nil {
			return nil, nil
		}
		clone := *p
		return &clone, nil
	case *types.AnyElement:
		if p == nil {
			return nil, nil
		}
		clone := *p
		return &clone, nil
	default:
		return particle, nil
	}
}
