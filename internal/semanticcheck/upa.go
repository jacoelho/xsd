package semanticcheck

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemaops"
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

	expandOptions := schemaops.ExpandGroupRefsOptions{
		Lookup: func(ref *types.GroupRef) *types.ModelGroup {
			if schema == nil || ref == nil {
				return nil
			}
			return schema.Groups[ref.RefQName]
		},
		MissingError: func(ref types.QName) error {
			if schema == nil {
				return fmt.Errorf("group ref %s not resolved", ref)
			}
			return fmt.Errorf("group '%s' not found", ref)
		},
		CycleError: func(ref types.QName) error {
			return fmt.Errorf("circular group reference detected for %s", ref)
		},
		AllGroupMode: schemaops.AllGroupAsChoice,
		LeafClone:    schemaops.LeafClone,
	}

	if particle != nil {
		expanded, err := schemaops.ExpandGroupRefs(particle, expandOptions)
		if err != nil {
			return err
		}
		particle = expanded
		particle = relaxOccursCopy(particle)
	}
	if baseParticle != nil {
		expanded, err := schemaops.ExpandGroupRefs(baseParticle, expandOptions)
		if err != nil {
			return err
		}
		baseParticle = expanded
		baseParticle = relaxOccursCopy(baseParticle)
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

func relaxOccursCopy(particle types.Particle) types.Particle {
	switch typed := particle.(type) {
	case *types.ElementDecl:
		if typed == nil {
			return nil
		}
		clone := *typed
		clone.MinOccurs, clone.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		return &clone
	case *types.AnyElement:
		if typed == nil {
			return nil
		}
		clone := *typed
		clone.MinOccurs, clone.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		return &clone
	case *types.ModelGroup:
		if typed == nil {
			return nil
		}
		clone := *typed
		clone.MinOccurs, clone.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		if len(typed.Particles) > 0 {
			clone.Particles = make([]types.Particle, 0, len(typed.Particles))
			for _, child := range typed.Particles {
				clone.Particles = append(clone.Particles, relaxOccursCopy(child))
			}
		}
		return &clone
	default:
		return particle
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
