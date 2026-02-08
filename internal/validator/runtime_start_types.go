package validator

import "github.com/jacoelho/xsd/internal/runtime"

type StartMatchKind uint8

const (
	MatchNone StartMatchKind = iota
	MatchElem
	MatchWildcard
)

type StartMatch struct {
	Kind     StartMatchKind
	Elem     runtime.ElemID
	Wildcard runtime.WildcardID
}

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

type StartResult struct {
	Elem   runtime.ElemID
	Type   runtime.TypeID
	Nilled bool
	Skip   bool
}
