package validator

import "github.com/jacoelho/xsd/internal/runtime"

// StartMatchKind defines an exported type.
type StartMatchKind uint8

const (
	// MatchNone is an exported constant.
	MatchNone StartMatchKind = iota
	// MatchElem is an exported constant.
	MatchElem
	// MatchWildcard is an exported constant.
	MatchWildcard
)

// StartMatch defines an exported type.
type StartMatch struct {
	Kind     StartMatchKind
	Elem     runtime.ElemID
	Wildcard runtime.WildcardID
}

// StartAttr defines an exported type.
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

// StartResult defines an exported type.
type StartResult struct {
	Elem   runtime.ElemID
	Type   runtime.TypeID
	Nilled bool
	Skip   bool
}
