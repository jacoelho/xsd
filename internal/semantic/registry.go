package semantic

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
	localElements   map[*types.ElementDecl]ElemID
	localAttributes map[*types.AttributeDecl]AttrID
	anonymousTypes  map[types.Type]TypeID
	TypeOrder       []TypeEntry
	ElementOrder    []ElementEntry
	AttributeOrder  []AttributeEntry
}

func newRegistry() *Registry {
	return &Registry{
		Types:           make(map[types.QName]TypeID),
		Elements:        make(map[types.QName]ElemID),
		Attributes:      make(map[types.QName]AttrID),
		TypeOrder:       []TypeEntry{},
		ElementOrder:    []ElementEntry{},
		AttributeOrder:  []AttributeEntry{},
		localElements:   make(map[*types.ElementDecl]ElemID),
		localAttributes: make(map[*types.AttributeDecl]AttrID),
		anonymousTypes:  make(map[types.Type]TypeID),
	}
}

// LookupLocalElementID resolves a local element declaration to its assigned ID.
func (r *Registry) LookupLocalElementID(decl *types.ElementDecl) (ElemID, bool) {
	if r == nil || decl == nil {
		return 0, false
	}
	id, ok := r.localElements[decl]
	return id, ok
}

// LookupLocalAttributeID resolves a local attribute declaration to its assigned ID.
func (r *Registry) LookupLocalAttributeID(decl *types.AttributeDecl) (AttrID, bool) {
	if r == nil || decl == nil {
		return 0, false
	}
	id, ok := r.localAttributes[decl]
	return id, ok
}

// LookupAnonymousTypeID resolves an anonymous type definition to its assigned ID.
func (r *Registry) LookupAnonymousTypeID(typ types.Type) (TypeID, bool) {
	if r == nil || typ == nil {
		return 0, false
	}
	id, ok := r.anonymousTypes[typ]
	return id, ok
}
