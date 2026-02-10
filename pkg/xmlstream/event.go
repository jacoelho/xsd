package xmlstream

import (
	"bytes"
	"fmt"

	"github.com/jacoelho/xsd/internal/qname"
)

// QName represents a namespace-qualified name.
type QName = qname.QName

// EventKind identifies the kind of streaming XML event.
type EventKind int

const (
	EventStartElement EventKind = iota
	EventEndElement
	EventCharData
	EventComment
	EventPI
	EventDirective
)

// String returns a readable name for the event kind.
func (k EventKind) String() string {
	switch k {
	case EventStartElement:
		return "StartElement"
	case EventEndElement:
		return "EndElement"
	case EventCharData:
		return "CharData"
	case EventComment:
		return "Comment"
	case EventPI:
		return "PI"
	case EventDirective:
		return "Directive"
	default:
		return fmt.Sprintf("EventKind(%d)", int(k))
	}
}

// ElementID is a monotonic identifier assigned per document.
type ElementID uint64

// NameID is a monotonic identifier assigned per document for expanded names.
type NameID uint32

// Event represents a single streaming XML token.
// Text and Attr.Value are valid until the next Next call.
type Event struct {
	Name       QName
	Attrs      []Attr
	Text       []byte
	Kind       EventKind
	Line       int
	Column     int
	ID         ElementID
	ScopeDepth int
}

// Attr holds a namespace-qualified attribute.
type Attr struct {
	Name  QName
	Value []byte
}

// ResolvedEvent represents a streaming XML token with namespace-resolved bytes.
// NS/Local/Attr slices are valid until the next NextResolved or Next call.
type ResolvedEvent struct {
	NS         []byte
	Local      []byte
	Attrs      []ResolvedAttr
	Text       []byte
	Kind       EventKind
	Line       int
	Column     int
	ID         ElementID
	ScopeDepth int
	NameID     NameID
}

// ResolvedAttr holds a namespace-resolved attribute.
type ResolvedAttr struct {
	NS     []byte
	Local  []byte
	Value  []byte
	NameID NameID
}

// RawName holds a raw QName split into prefix and local parts.
// The byte slices are valid until the next Next or NextRaw call.
type RawName struct {
	Full   []byte
	Prefix []byte
	Local  []byte
}

// HasLocal reports whether the local name matches.
func (n RawName) HasLocal(local []byte) bool {
	return bytes.Equal(n.Local, local)
}

// RawAttr holds a raw attribute name and value.
// The byte slices are valid until the next Next or NextRaw call.
type RawAttr struct {
	Name  RawName
	Value []byte
}

// RawEvent represents a streaming XML token with raw names.
// The byte slices are valid until the next Next or NextRaw call.
type RawEvent struct {
	Name       RawName
	Attrs      []RawAttr
	Text       []byte
	Kind       EventKind
	Line       int
	Column     int
	ID         ElementID
	ScopeDepth int
}

// Attr returns the raw attribute value by local name and optional prefix.
func (e RawEvent) Attr(prefix, local []byte) ([]byte, bool) {
	for _, attr := range e.Attrs {
		if bytes.Equal(attr.Name.Prefix, prefix) && bytes.Equal(attr.Name.Local, local) {
			return attr.Value, true
		}
	}
	return nil, false
}

// AttrLocal returns the raw attribute value by local name, ignoring prefix.
func (e RawEvent) AttrLocal(local []byte) ([]byte, bool) {
	for _, attr := range e.Attrs {
		if bytes.Equal(attr.Name.Local, local) {
			return attr.Value, true
		}
	}
	return nil, false
}

// Attr returns the attribute value by namespace and local name.
func (e Event) Attr(namespace, local string) ([]byte, bool) {
	for _, attr := range e.Attrs {
		if attr.Name.Namespace == namespace && attr.Name.Local == local {
			return attr.Value, true
		}
	}
	return nil, false
}

// AttrLocal returns the attribute value by local name, ignoring namespace.
func (e Event) AttrLocal(local string) ([]byte, bool) {
	for _, attr := range e.Attrs {
		if attr.Name.Local == local {
			return attr.Value, true
		}
	}
	return nil, false
}
