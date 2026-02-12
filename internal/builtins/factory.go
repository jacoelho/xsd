package builtins

import (
	schematypes "github.com/jacoelho/xsd/internal/types"
)

func NewSimpleType(name schematypes.TypeName) (*schematypes.SimpleType, error) {
	return schematypes.NewBuiltinSimpleType(name)
}
