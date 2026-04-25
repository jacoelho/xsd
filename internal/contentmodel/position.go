package contentmodel

import "github.com/jacoelho/xsd/internal/runtime"

// PositionKind identifies the kind of Glushkov position.
type PositionKind uint8

const (
	PositionElement PositionKind = iota
	PositionWildcard
)

// Position represents a single element or wildcard occurrence.
type Position struct {
	Element     any
	Wildcard    any
	ElementID   uint32
	WildcardID  uint32
	Kind        PositionKind
	AllowsSubst bool
	RuntimeRule bool
}

// Glushkov contains the compiled position sets and followpos relations.
type Glushkov struct {
	firstRaw  *bitset
	lastRaw   *bitset
	Positions []Position
	Follow    []runtime.BitsetRef
	Bitsets   runtime.BitsetBlob
	followRaw []*bitset
	First     runtime.BitsetRef
	Last      runtime.BitsetRef
	Nullable  bool
}
