package runtime

// ElementStartInfo is the runtime-published data needed to start validating an
// element declaration.
type ElementStartInfo struct {
	Type     TypeID
	Block    DerivationMask
	Abstract bool
	Nillable bool
	Fixed    bool
	Default  bool
}

// ElementStartInfoShape is the schema-independent projection used to publish
// element start metadata.
type ElementStartInfoShape struct {
	Type     TypeID
	Block    DerivationMask
	Abstract bool
	Nillable bool
	Fixed    bool
	Default  bool
}

// NewElementStartInfo returns the start projection for one element
// declaration.
func NewElementStartInfo(shape ElementStartInfoShape) ElementStartInfo {
	return ElementStartInfo(shape)
}

// EqualElementStartInfo reports whether two element start projections expose
// the same runtime-facing facts.
func EqualElementStartInfo(a, b ElementStartInfo) bool {
	return a == b
}

// TypeInfo is the runtime-published data needed to start validating a runtime
// type.
type TypeInfo struct {
	Block       DerivationMask
	Abstract    bool
	Unavailable bool
}

// TypeInfoShape is the schema-independent projection used to publish type
// start metadata.
type TypeInfoShape struct {
	Block       DerivationMask
	Abstract    bool
	Unavailable bool
}

// NewTypeInfo returns the start projection for one runtime type.
func NewTypeInfo(shape TypeInfoShape) TypeInfo {
	return TypeInfo(shape)
}
