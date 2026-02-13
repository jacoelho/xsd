package analysis

import (
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/types"
)

const (
	// InvalidTypeID represents an unassigned type ID.
	InvalidTypeID ids.TypeID = ids.InvalidTypeID
	// InvalidElemID represents an unassigned element ID.
	InvalidElemID ids.ElemID = ids.InvalidElemID
	// InvalidAttrID represents an unassigned attribute ID.
	InvalidAttrID ids.AttrID = ids.InvalidAttrID
)

// TypeEntry records a type ID assignment in traversal order.
type TypeEntry struct {
	Type   types.Type
	QName  types.QName
	ID     ids.TypeID
	Global bool
}

// ElementEntry records an element ID assignment in traversal order.
type ElementEntry struct {
	Decl   *types.ElementDecl
	QName  types.QName
	ID     ids.ElemID
	Global bool
}

// AttributeEntry records an attribute ID assignment in traversal order.
type AttributeEntry struct {
	Decl   *types.AttributeDecl
	QName  types.QName
	ID     ids.AttrID
	Global bool
}

// Registry holds deterministic ID assignments for schema components.
type Registry struct {
	Types           map[types.QName]ids.TypeID
	Elements        map[types.QName]ids.ElemID
	Attributes      map[types.QName]ids.AttrID
	localElements   map[*types.ElementDecl]ids.ElemID
	localAttributes map[*types.AttributeDecl]ids.AttrID
	anonymousTypes  map[types.Type]ids.TypeID
	TypeOrder       []TypeEntry
	ElementOrder    []ElementEntry
	AttributeOrder  []AttributeEntry
}

func newRegistry() *Registry {
	return &Registry{
		Types:           make(map[types.QName]ids.TypeID),
		Elements:        make(map[types.QName]ids.ElemID),
		Attributes:      make(map[types.QName]ids.AttrID),
		TypeOrder:       []TypeEntry{},
		ElementOrder:    []ElementEntry{},
		AttributeOrder:  []AttributeEntry{},
		localElements:   make(map[*types.ElementDecl]ids.ElemID),
		localAttributes: make(map[*types.AttributeDecl]ids.AttrID),
		anonymousTypes:  make(map[types.Type]ids.TypeID),
	}
}

// LookupLocalElementID resolves a local element declaration to its assigned ID.
func (r *Registry) LookupLocalElementID(decl *types.ElementDecl) (ids.ElemID, bool) {
	if r == nil || decl == nil {
		return 0, false
	}
	id, ok := r.localElements[decl]
	return id, ok
}

// LookupLocalAttributeID resolves a local attribute declaration to its assigned ID.
func (r *Registry) LookupLocalAttributeID(decl *types.AttributeDecl) (ids.AttrID, bool) {
	if r == nil || decl == nil {
		return 0, false
	}
	id, ok := r.localAttributes[decl]
	return id, ok
}

// LookupAnonymousTypeID resolves an anonymous type definition to its assigned ID.
func (r *Registry) LookupAnonymousTypeID(typ types.Type) (ids.TypeID, bool) {
	if r == nil || typ == nil {
		return 0, false
	}
	id, ok := r.anonymousTypes[typ]
	return id, ok
}
