package analysis

import (
	"github.com/jacoelho/xsd/internal/ids"
	model "github.com/jacoelho/xsd/internal/types"
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
	Type   model.Type
	QName  model.QName
	ID     ids.TypeID
	Global bool
}

// ElementEntry records an element ID assignment in traversal order.
type ElementEntry struct {
	Decl   *model.ElementDecl
	QName  model.QName
	ID     ids.ElemID
	Global bool
}

// AttributeEntry records an attribute ID assignment in traversal order.
type AttributeEntry struct {
	Decl   *model.AttributeDecl
	QName  model.QName
	ID     ids.AttrID
	Global bool
}

// Registry holds deterministic ID assignments for schema components.
type Registry struct {
	Types           map[model.QName]ids.TypeID
	Elements        map[model.QName]ids.ElemID
	Attributes      map[model.QName]ids.AttrID
	localElements   map[*model.ElementDecl]ids.ElemID
	localAttributes map[*model.AttributeDecl]ids.AttrID
	anonymousTypes  map[model.Type]ids.TypeID
	TypeOrder       []TypeEntry
	ElementOrder    []ElementEntry
	AttributeOrder  []AttributeEntry
}

func newRegistry() *Registry {
	return &Registry{
		Types:           make(map[model.QName]ids.TypeID),
		Elements:        make(map[model.QName]ids.ElemID),
		Attributes:      make(map[model.QName]ids.AttrID),
		TypeOrder:       []TypeEntry{},
		ElementOrder:    []ElementEntry{},
		AttributeOrder:  []AttributeEntry{},
		localElements:   make(map[*model.ElementDecl]ids.ElemID),
		localAttributes: make(map[*model.AttributeDecl]ids.AttrID),
		anonymousTypes:  make(map[model.Type]ids.TypeID),
	}
}

// LookupLocalElementID resolves a local element declaration to its assigned ID.
func (r *Registry) LookupLocalElementID(decl *model.ElementDecl) (ids.ElemID, bool) {
	if r == nil || decl == nil {
		return 0, false
	}
	id, ok := r.localElements[decl]
	return id, ok
}

// LookupLocalAttributeID resolves a local attribute declaration to its assigned ID.
func (r *Registry) LookupLocalAttributeID(decl *model.AttributeDecl) (ids.AttrID, bool) {
	if r == nil || decl == nil {
		return 0, false
	}
	id, ok := r.localAttributes[decl]
	return id, ok
}

// LookupAnonymousTypeID resolves an anonymous type definition to its assigned ID.
func (r *Registry) LookupAnonymousTypeID(typ model.Type) (ids.TypeID, bool) {
	if r == nil || typ == nil {
		return 0, false
	}
	id, ok := r.anonymousTypes[typ]
	return id, ok
}
