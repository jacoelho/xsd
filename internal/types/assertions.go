package types

func as[T any](value any) (T, bool) {
	v, ok := value.(T)
	return v, ok
}

// AsSimpleType performs a type assertion to *SimpleType.
func AsSimpleType(t Type) (*SimpleType, bool) {
	return as[*SimpleType](t)
}

// AsComplexType performs a type assertion to *ComplexType.
func AsComplexType(t Type) (*ComplexType, bool) {
	return as[*ComplexType](t)
}

// AsBuiltinType performs a type assertion to *BuiltinType.
func AsBuiltinType(t Type) (*BuiltinType, bool) {
	return as[*BuiltinType](t)
}

// AsDerivedType performs a type assertion to DerivedType.
func AsDerivedType(t Type) (DerivedType, bool) {
	return as[DerivedType](t)
}
