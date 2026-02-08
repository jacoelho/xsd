package parser

import "github.com/jacoelho/xsd/internal/types"

func hasPlaceholderInContent(content types.Content, visitedTypes map[types.Type]bool, visitedGroups map[*types.ModelGroup]bool) bool {
	switch c := content.(type) {
	case *types.ElementContent:
		return hasPlaceholderInParticle(c.Particle, visitedTypes, visitedGroups)
	case *types.ComplexContent:
		if c.Extension != nil && hasPlaceholderInParticle(c.Extension.Particle, visitedTypes, visitedGroups) {
			return true
		}
		if c.Restriction != nil && hasPlaceholderInParticle(c.Restriction.Particle, visitedTypes, visitedGroups) {
			return true
		}
	case *types.SimpleContent:
		if c.Extension != nil {
			for _, attr := range c.Extension.Attributes {
				if attr == nil {
					continue
				}
				if hasPlaceholderType(attr.Type, visitedTypes, visitedGroups) {
					return true
				}
			}
		}
		if c.Restriction != nil {
			if c.Restriction.SimpleType != nil && hasPlaceholderType(c.Restriction.SimpleType, visitedTypes, visitedGroups) {
				return true
			}
			for _, attr := range c.Restriction.Attributes {
				if attr == nil {
					continue
				}
				if hasPlaceholderType(attr.Type, visitedTypes, visitedGroups) {
					return true
				}
			}
		}
	}
	return false
}

func hasPlaceholderInModelGroup(group *types.ModelGroup, visitedTypes map[types.Type]bool, visitedGroups map[*types.ModelGroup]bool) bool {
	if group == nil {
		return false
	}
	if visitedGroups[group] {
		return false
	}
	visitedGroups[group] = true
	for _, particle := range group.Particles {
		if hasPlaceholderInParticle(particle, visitedTypes, visitedGroups) {
			return true
		}
	}
	return false
}

func hasPlaceholderInParticle(particle types.Particle, visitedTypes map[types.Type]bool, visitedGroups map[*types.ModelGroup]bool) bool {
	switch p := particle.(type) {
	case *types.ElementDecl:
		if p == nil {
			return false
		}
		return hasPlaceholderType(p.Type, visitedTypes, visitedGroups)
	case *types.ModelGroup:
		return hasPlaceholderInModelGroup(p, visitedTypes, visitedGroups)
	default:
		return false
	}
}
