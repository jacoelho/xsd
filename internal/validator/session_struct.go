package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// Session holds per-document runtime validation state.
type Session struct {
	SessionIO
	SessionBuffers
	SessionIdentity
	AttributeTracker

	Names            NameState
	rt               *runtime.Schema
	Scratch          Scratch
	elemStack        []elemFrame
	validationErrors []xsderrors.Validation
	Arena            Arena
	normDepth        int
}
