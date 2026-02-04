package parser

import "github.com/jacoelho/xsd/internal/types"

// UpdatePlaceholderState recalculates and stores placeholder presence.
func UpdatePlaceholderState(schema *Schema) {
	if schema == nil {
		return
	}
	schema.HasPlaceholders = hasPlaceholders(schema)
}

func hasPlaceholders(schema *Schema) bool {
	if schema == nil {
		return false
	}
	visitedTypes := make(map[types.Type]bool)
	visitedGroups := make(map[*types.ModelGroup]bool)

	for _, typ := range schema.TypeDefs {
		if hasPlaceholderType(typ, visitedTypes, visitedGroups) {
			return true
		}
	}
	for _, decl := range schema.ElementDecls {
		if decl == nil {
			continue
		}
		if hasPlaceholderType(decl.Type, visitedTypes, visitedGroups) {
			return true
		}
	}
	for _, decl := range schema.AttributeDecls {
		if decl == nil {
			continue
		}
		if hasPlaceholderType(decl.Type, visitedTypes, visitedGroups) {
			return true
		}
	}
	for _, group := range schema.Groups {
		if hasPlaceholderInModelGroup(group, visitedTypes, visitedGroups) {
			return true
		}
	}
	for _, group := range schema.AttributeGroups {
		if hasPlaceholderInAttributeGroup(group, visitedTypes, visitedGroups) {
			return true
		}
	}
	return false
}

func hasPlaceholderInAttributeGroup(group *types.AttributeGroup, visitedTypes map[types.Type]bool, visitedGroups map[*types.ModelGroup]bool) bool {
	if group == nil {
		return false
	}
	for _, attr := range group.Attributes {
		if attr == nil {
			continue
		}
		if hasPlaceholderType(attr.Type, visitedTypes, visitedGroups) {
			return true
		}
	}
	return false
}

func hasPlaceholderType(typ types.Type, visitedTypes map[types.Type]bool, visitedGroups map[*types.ModelGroup]bool) bool {
	if typ == nil {
		return false
	}
	if visitedTypes[typ] {
		return false
	}
	visitedTypes[typ] = true

	switch t := typ.(type) {
	case *types.SimpleType:
		if types.IsPlaceholderSimpleType(t) {
			return true
		}
		if t.Restriction != nil {
			if t.ResolvedBase != nil && hasPlaceholderType(t.ResolvedBase, visitedTypes, visitedGroups) {
				return true
			}
			if t.Restriction.SimpleType != nil && hasPlaceholderType(t.Restriction.SimpleType, visitedTypes, visitedGroups) {
				return true
			}
		}
		if t.List != nil {
			if t.ItemType != nil && hasPlaceholderType(t.ItemType, visitedTypes, visitedGroups) {
				return true
			}
			if t.List.InlineItemType != nil && hasPlaceholderType(t.List.InlineItemType, visitedTypes, visitedGroups) {
				return true
			}
		}
		if t.Union != nil {
			for _, member := range t.MemberTypes {
				if hasPlaceholderType(member, visitedTypes, visitedGroups) {
					return true
				}
			}
			for _, inline := range t.Union.InlineTypes {
				if hasPlaceholderType(inline, visitedTypes, visitedGroups) {
					return true
				}
			}
		}
	case *types.ComplexType:
		if t.ResolvedBase != nil && hasPlaceholderType(t.ResolvedBase, visitedTypes, visitedGroups) {
			return true
		}
		if hasPlaceholderInContent(t.Content(), visitedTypes, visitedGroups) {
			return true
		}
		for _, attr := range t.Attributes() {
			if attr == nil {
				continue
			}
			if hasPlaceholderType(attr.Type, visitedTypes, visitedGroups) {
				return true
			}
		}
	}

	return false
}

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
