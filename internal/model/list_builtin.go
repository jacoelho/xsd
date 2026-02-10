package model

import "github.com/jacoelho/xsd/internal/listtypes"

// isBuiltinListTypeName reports whether the name is one of the built-in list datatypes.
func isBuiltinListTypeName(name string) bool {
	return listtypes.IsTypeName(name)
}

// builtinListItemTypeName returns the item type name for built-in list datatypes.
func builtinListItemTypeName(name string) (TypeName, bool) {
	item, ok := listtypes.ItemTypeName(name)
	return TypeName(item), ok
}
