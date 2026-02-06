package ids

// TypeID identifies a type definition.
type TypeID uint32

// ElemID identifies an element declaration.
type ElemID uint32

// AttrID identifies an attribute declaration.
type AttrID uint32

const (
	// InvalidTypeID represents an unassigned type ID.
	InvalidTypeID TypeID = 0
	// InvalidElemID represents an unassigned element ID.
	InvalidElemID ElemID = 0
	// InvalidAttrID represents an unassigned attribute ID.
	InvalidAttrID AttrID = 0
)
