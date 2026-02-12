package builtins

import schematypes "github.com/jacoelho/xsd/internal/types"

var builtinListItemTypes = map[schematypes.TypeName]schematypes.TypeName{
	TypeNameNMTOKENS: TypeNameNMTOKEN,
	TypeNameIDREFS:   TypeNameIDREF,
	TypeNameENTITIES: TypeNameENTITY,
}

func IsBuiltinListTypeName(name string) bool {
	_, ok := BuiltinListItemTypeName(name)
	return ok
}

func BuiltinListItemTypeName(name string) (schematypes.TypeName, bool) {
	item, ok := builtinListItemTypes[schematypes.TypeName(name)]
	return item, ok
}
