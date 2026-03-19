package model

import "github.com/jacoelho/xsd/internal/runtime"

// MatchKind enumerates content-model match results.
type MatchKind uint8

const (
	MatchNone MatchKind = iota
	MatchElem
	MatchWildcard
)

// Match describes the matched particle for one content-model step.
type Match struct {
	Kind     MatchKind
	Elem     runtime.ElemID
	Wildcard runtime.WildcardID
}

// State tracks the runtime state of one compiled content model.
type State struct {
	NFA        []uint64
	nfaScratch []uint64
	All        []uint64
	DFA        uint32
	AllCount   uint32
	Kind       runtime.ModelKind
}
