package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

type identityState struct {
	arena *Arena
	State[RuntimeFrame]
}

type identityStartInput struct {
	Attrs   []Start
	Applied []Applied
	Elem    runtime.ElemID
	Type    runtime.TypeID
	Sym     runtime.SymbolID
	NS      runtime.NamespaceID
	Nilled  bool
}

type identityEndInput struct {
	Text      []byte
	KeyBytes  []byte
	TextState TextState
	KeyKind   runtime.ValueKind
}
