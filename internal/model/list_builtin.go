package model

var builtinListItemTypes = map[TypeName]TypeName{
	TypeNameNMTOKENS: TypeNameNMTOKEN,
	TypeNameIDREFS:   TypeNameIDREF,
	TypeNameENTITIES: TypeNameENTITY,
}

// isBuiltinListTypeName reports whether the name is one of the built-in list datatypes.
func isBuiltinListTypeName(name string) bool {
	_, ok := builtinListItemTypeName(name)
	return ok
}

// builtinListItemTypeName returns the item type name for built-in list datatypes.
func builtinListItemTypeName(name string) (TypeName, bool) {
	item, ok := builtinListItemTypes[TypeName(name)]
	return item, ok
}

// IsBuiltinListTypeName reports whether name is a built-in list datatype.
func IsBuiltinListTypeName(name string) bool {
	return isBuiltinListTypeName(name)
}

// BuiltinListItemTypeName returns the built-in item type name for name.
func BuiltinListItemTypeName(name string) (TypeName, bool) {
	return builtinListItemTypeName(name)
}
