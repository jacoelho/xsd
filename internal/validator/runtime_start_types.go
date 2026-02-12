package validator

import "github.com/jacoelho/xsd/internal/runtime"

// StartMatchKind enumerates start match kind values.
type StartMatchKind uint8

const (
	MatchNone StartMatchKind = iota
	MatchElem
	MatchWildcard
)

// StartMatch describes the matched particle for a start-element step.
type StartMatch struct {
	Kind     StartMatchKind
	Elem     runtime.ElemID
	Wildcard runtime.WildcardID
}

// StartAttr carries one start-element attribute and its interned metadata.
type StartAttr struct {
	NSBytes    []byte
	Local      []byte
	Value      []byte
	KeyBytes   []byte
	Sym        runtime.SymbolID
	NS         runtime.NamespaceID
	NameCached bool
	KeyKind    runtime.ValueKind
}

// StartResult describes the resolved element/type and xsi:nil handling for a start event.
type StartResult struct {
	Elem   runtime.ElemID
	Type   runtime.TypeID
	Nilled bool
	Skip   bool
}
