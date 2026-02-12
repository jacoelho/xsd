package builtins

import (
	schematypes "github.com/jacoelho/xsd/internal/types"
)

// NewSimpleType constructs a built-in simple type by XML Schema type name.
func NewSimpleType(name schematypes.TypeName) (*schematypes.SimpleType, error) {
	return schematypes.NewBuiltinSimpleType(name)
}
