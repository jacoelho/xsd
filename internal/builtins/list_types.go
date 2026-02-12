package builtins

import schematypes "github.com/jacoelho/xsd/internal/types"

var builtinListItemTypes = map[schematypes.TypeName]schematypes.TypeName{
	TypeNameNMTOKENS: TypeNameNMTOKEN,
	TypeNameIDREFS:   TypeNameIDREF,
	TypeNameENTITIES: TypeNameENTITY,
}

// IsBuiltinListTypeName is an exported function.
func IsBuiltinListTypeName(name string) bool {
	_, ok := BuiltinListItemTypeName(name)
	return ok
}

// BuiltinListItemTypeName is an exported function.
func BuiltinListItemTypeName(name string) (schematypes.TypeName, bool) {
	item, ok := builtinListItemTypes[schematypes.TypeName(name)]
	return item, ok
}
