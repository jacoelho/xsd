package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stack"
)

// Session holds per-document runtime validation state.
type Session struct {
	SessionIO
	SessionBuffers
	SessionIdentity
	AttributeTracker

	nameMapSparse    map[NameID]nameEntry
	rt               *runtime.Schema
	Scratch          Scratch
	nameMap          []nameEntry
	elemStack        []elemFrame
	validationErrors []xsderrors.Validation
	nsStack          stack.Stack[nsFrame]
	Arena            Arena
	normDepth        int
}
