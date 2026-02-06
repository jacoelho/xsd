package types

// IsBuiltinListTypeName reports whether the name is one of the built-in list datatypes.
func IsBuiltinListTypeName(name string) bool {
	_, ok := BuiltinListItemTypeName(name)
	return ok
}

// BuiltinListItemTypeName returns the item type name for built-in list datatypes.
func BuiltinListItemTypeName(name string) (TypeName, bool) {
	switch name {
	case string(TypeNameNMTOKENS):
		return TypeNameNMTOKEN, true
	case string(TypeNameIDREFS):
		return TypeNameIDREF, true
	case string(TypeNameENTITIES):
		return TypeNameENTITY, true
	default:
		return "", false
	}
}
