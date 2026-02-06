package schema

import (
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/types"
)

// TypeID identifies a type definition.
type TypeID = ids.TypeID

// ElemID identifies an element declaration.
type ElemID = ids.ElemID

// AttrID identifies an attribute declaration.
type AttrID = ids.AttrID

const (
	// InvalidTypeID represents an unassigned type ID.
	InvalidTypeID = ids.InvalidTypeID
	// InvalidElemID represents an unassigned element ID.
	InvalidElemID = ids.InvalidElemID
	// InvalidAttrID represents an unassigned attribute ID.
	InvalidAttrID = ids.InvalidAttrID
)

// TypeEntry records a type ID assignment in traversal order.
type TypeEntry struct {
	Type   types.Type
	QName  types.QName
	ID     TypeID
	Global bool
}

// ElementEntry records an element ID assignment in traversal order.
type ElementEntry struct {
	Decl   *types.ElementDecl
	QName  types.QName
	ID     ElemID
	Global bool
}

// AttributeEntry records an attribute ID assignment in traversal order.
type AttributeEntry struct {
	Decl   *types.AttributeDecl
	QName  types.QName
	ID     AttrID
	Global bool
}

// Registry holds deterministic ID assignments for schema components.
type Registry struct {
	Types           map[types.QName]TypeID
	Elements        map[types.QName]ElemID
	Attributes      map[types.QName]AttrID
	LocalElements   map[*types.ElementDecl]ElemID
	LocalAttributes map[*types.AttributeDecl]AttrID
	AnonymousTypes  map[types.Type]TypeID
	TypeOrder       []TypeEntry
	ElementOrder    []ElementEntry
	AttributeOrder  []AttributeEntry
}

func newRegistry() *Registry {
	return &Registry{
		Types:           make(map[types.QName]TypeID),
		Elements:        make(map[types.QName]ElemID),
		Attributes:      make(map[types.QName]AttrID),
		LocalElements:   make(map[*types.ElementDecl]ElemID),
		LocalAttributes: make(map[*types.AttributeDecl]AttrID),
		AnonymousTypes:  make(map[types.Type]TypeID),
		TypeOrder:       []TypeEntry{},
		ElementOrder:    []ElementEntry{},
		AttributeOrder:  []AttributeEntry{},
	}
}
