package parser

import "github.com/jacoelho/xsd/internal/types"

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
		if hasPlaceholderInSimpleType(t, visitedTypes, visitedGroups) {
			return true
		}
	case *types.ComplexType:
		if hasPlaceholderInComplexType(t, visitedTypes, visitedGroups) {
			return true
		}
	}

	return false
}

func hasPlaceholderInSimpleType(t *types.SimpleType, visitedTypes map[types.Type]bool, visitedGroups map[*types.ModelGroup]bool) bool {
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
	return false
}

func hasPlaceholderInComplexType(t *types.ComplexType, visitedTypes map[types.Type]bool, visitedGroups map[*types.ModelGroup]bool) bool {
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
