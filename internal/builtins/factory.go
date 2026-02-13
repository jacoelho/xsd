package builtins

import (
	"github.com/jacoelho/xsd/internal/types"
)

// NewSimpleType constructs a built-in simple type by XML Schema type name.
func NewSimpleType(name types.TypeName) (*types.SimpleType, error) {
	return types.NewBuiltinSimpleType(name)
}
