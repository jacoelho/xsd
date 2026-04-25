package validator

import "github.com/jacoelho/xsd/internal/runtime"

// StartMatchKind enumerates content-model match results.
type StartMatchKind uint8

const (
	StartMatchNone StartMatchKind = iota
	StartMatchElem
	StartMatchWildcard
)

// StartMatch describes the matched particle for one content-model step.
type StartMatch struct {
	Kind     StartMatchKind
	Elem     runtime.ElemID
	Wildcard runtime.WildcardID
}

// StartModelState tracks the runtime state of one compiled content schemaast.
type StartModelState struct {
	NFA        []uint64
	nfaScratch []uint64
	All        []uint64
	DFA        uint32
	AllCount   uint32
	Kind       runtime.ModelKind
}
