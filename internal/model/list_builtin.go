package model

import "github.com/jacoelho/xsd/internal/builtinlist"

// isBuiltinListTypeName reports whether the name is one of the built-in list datatypes.
func isBuiltinListTypeName(name string) bool {
	return builtinlist.IsTypeName(name)
}

// builtinListItemTypeName returns the item type name for built-in list datatypes.
func builtinListItemTypeName(name string) (TypeName, bool) {
	item, ok := builtinlist.ItemTypeName(name)
	return TypeName(item), ok
}
