package schemaanalysis

import (
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/model"
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
	Type   model.Type
	QName  model.QName
	ID     TypeID
	Global bool
}

// ElementEntry records an element ID assignment in traversal order.
type ElementEntry struct {
	Decl   *model.ElementDecl
	QName  model.QName
	ID     ElemID
	Global bool
}

// AttributeEntry records an attribute ID assignment in traversal order.
type AttributeEntry struct {
	Decl   *model.AttributeDecl
	QName  model.QName
	ID     AttrID
	Global bool
}

// Registry holds deterministic ID assignments for schema components.
type Registry struct {
	Types           map[model.QName]TypeID
	Elements        map[model.QName]ElemID
	Attributes      map[model.QName]AttrID
	localElements   map[*model.ElementDecl]ElemID
	localAttributes map[*model.AttributeDecl]AttrID
	anonymousTypes  map[model.Type]TypeID
	TypeOrder       []TypeEntry
	ElementOrder    []ElementEntry
	AttributeOrder  []AttributeEntry
}

func newRegistry() *Registry {
	return &Registry{
		Types:           make(map[model.QName]TypeID),
		Elements:        make(map[model.QName]ElemID),
		Attributes:      make(map[model.QName]AttrID),
		TypeOrder:       []TypeEntry{},
		ElementOrder:    []ElementEntry{},
		AttributeOrder:  []AttributeEntry{},
		localElements:   make(map[*model.ElementDecl]ElemID),
		localAttributes: make(map[*model.AttributeDecl]AttrID),
		anonymousTypes:  make(map[model.Type]TypeID),
	}
}

// LookupLocalElementID resolves a local element declaration to its assigned ID.
func (r *Registry) LookupLocalElementID(decl *model.ElementDecl) (ElemID, bool) {
	if r == nil || decl == nil {
		return 0, false
	}
	id, ok := r.localElements[decl]
	return id, ok
}

// LookupLocalAttributeID resolves a local attribute declaration to its assigned ID.
func (r *Registry) LookupLocalAttributeID(decl *model.AttributeDecl) (AttrID, bool) {
	if r == nil || decl == nil {
		return 0, false
	}
	id, ok := r.localAttributes[decl]
	return id, ok
}

// LookupAnonymousTypeID resolves an anonymous type definition to its assigned ID.
func (r *Registry) LookupAnonymousTypeID(typ model.Type) (TypeID, bool) {
	if r == nil || typ == nil {
		return 0, false
	}
	id, ok := r.anonymousTypes[typ]
	return id, ok
}
