package parser

import "github.com/jacoelho/xsd/internal/types"

// HasPlaceholders reports whether unresolved type placeholders remain.
func HasPlaceholders(schema *Schema) bool {
	return hasPlaceholders(schema)
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
