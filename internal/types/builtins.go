package types

import "github.com/jacoelho/xsd/internal/model"

// BuiltinTypes returns built-in XSD types in deterministic order.
func BuiltinTypes() []*BuiltinType {
	return model.BuiltinTypes()
}

// NewBuiltinSimpleType creates a built-in simple type by name.
func NewBuiltinSimpleType(name TypeName) (*SimpleType, error) {
	return model.NewBuiltinSimpleType(name)
}
