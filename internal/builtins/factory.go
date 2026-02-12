package builtins

import (
	schematypes "github.com/jacoelho/xsd/internal/types"
)

// NewSimpleType is an exported function.
func NewSimpleType(name schematypes.TypeName) (*schematypes.SimpleType, error) {
	return schematypes.NewBuiltinSimpleType(name)
}
