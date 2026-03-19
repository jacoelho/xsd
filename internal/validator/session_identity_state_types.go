package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/identity"
)

type identityState struct {
	arena *Arena
	identity.State[identity.RuntimeFrame]
}

type identityStartInput struct {
	Attrs   []attrs.Start
	Applied []attrs.Applied
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
