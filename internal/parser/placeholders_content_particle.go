package parser

import "github.com/jacoelho/xsd/internal/model"

func hasPlaceholderInContent(content model.Content, visitedTypes map[model.Type]bool, visitedGroups map[*model.ModelGroup]bool) bool {
	switch c := content.(type) {
	case *model.ElementContent:
		return hasPlaceholderInParticle(c.Particle, visitedTypes, visitedGroups)
	case *model.ComplexContent:
		if c.Extension != nil && hasPlaceholderInParticle(c.Extension.Particle, visitedTypes, visitedGroups) {
			return true
		}
		if c.Restriction != nil && hasPlaceholderInParticle(c.Restriction.Particle, visitedTypes, visitedGroups) {
			return true
		}
	case *model.SimpleContent:
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

func hasPlaceholderInModelGroup(group *model.ModelGroup, visitedTypes map[model.Type]bool, visitedGroups map[*model.ModelGroup]bool) bool {
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

func hasPlaceholderInParticle(particle model.Particle, visitedTypes map[model.Type]bool, visitedGroups map[*model.ModelGroup]bool) bool {
	switch p := particle.(type) {
	case *model.ElementDecl:
		if p == nil {
			return false
		}
		return hasPlaceholderType(p.Type, visitedTypes, visitedGroups)
	case *model.ModelGroup:
		return hasPlaceholderInModelGroup(p, visitedTypes, visitedGroups)
	default:
		return false
	}
}
