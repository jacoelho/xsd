package builtins

import schematypes "github.com/jacoelho/xsd/internal/types"

type registry struct {
	byName  map[schematypes.TypeName]*schematypes.BuiltinType
	ordered []*schematypes.BuiltinType
}

var defaultRegistry = newRegistry(schematypes.BuiltinTypes())
