package builtins

import "github.com/jacoelho/xsd/internal/types"

type registry struct {
	byName  map[types.TypeName]*types.BuiltinType
	ordered []*types.BuiltinType
}

var defaultRegistry = newRegistry(types.BuiltinTypes())
