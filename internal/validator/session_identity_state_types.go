package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stack"
)

type identityState struct {
	arena  *Arena
	frames stack.Stack[rtIdentityFrame]
	scopes stack.Stack[rtIdentityScope]
	// uncommittedViolations are immediate identity processing failures tied to the active frame.
	uncommittedViolations []error
	// committedViolations are scope-finalized violations emitted on the next event boundary.
	committedViolations []error
	nextNodeID          uint64
	active              bool
}

type identitySnapshot struct {
	nextNodeID  uint64
	framesLen   int
	scopesLen   int
	uncommitted int
	committed   int
	active      bool
}

type identityStartInput struct {
	Attrs   []StartAttr
	Applied []AttrApplied
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
