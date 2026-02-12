package builtins

import schematypes "github.com/jacoelho/xsd/internal/types"

var builtinListItemTypes = map[schematypes.TypeName]schematypes.TypeName{
	TypeNameNMTOKENS: TypeNameNMTOKEN,
	TypeNameIDREFS:   TypeNameIDREF,
	TypeNameENTITIES: TypeNameENTITY,
}

// IsBuiltinListTypeName reports whether name is one of the built-in list simple types.
func IsBuiltinListTypeName(name string) bool {
	_, ok := BuiltinListItemTypeName(name)
	return ok
}

// BuiltinListItemTypeName returns the built-in item type for a built-in list type.
func BuiltinListItemTypeName(name string) (schematypes.TypeName, bool) {
	item, ok := builtinListItemTypes[schematypes.TypeName(name)]
	return item, ok
}
