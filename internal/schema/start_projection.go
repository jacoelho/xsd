package schema

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

// TypeInfo is the runtime-published data needed to start validating a runtime
// type.
type TypeInfo struct {
	Block       DerivationMask
	Abstract    bool
	Unavailable bool
}
