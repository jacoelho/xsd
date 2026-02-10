package parser

import "github.com/jacoelho/xsd/internal/model"

func hasPlaceholderType(typ model.Type, visitedTypes map[model.Type]bool, visitedGroups map[*model.ModelGroup]bool) bool {
	if typ == nil {
		return false
	}
	if visitedTypes[typ] {
		return false
	}
	visitedTypes[typ] = true

	switch t := typ.(type) {
	case *model.SimpleType:
		if hasPlaceholderInSimpleType(t, visitedTypes, visitedGroups) {
			return true
		}
	case *model.ComplexType:
		if hasPlaceholderInComplexType(t, visitedTypes, visitedGroups) {
			return true
		}
	}

	return false
}

func hasPlaceholderInSimpleType(t *model.SimpleType, visitedTypes map[model.Type]bool, visitedGroups map[*model.ModelGroup]bool) bool {
	if model.IsPlaceholderSimpleType(t) {
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
	return false
}

func hasPlaceholderInComplexType(t *model.ComplexType, visitedTypes map[model.Type]bool, visitedGroups map[*model.ModelGroup]bool) bool {
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
	return false
}
