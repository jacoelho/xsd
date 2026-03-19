package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/names"
)

// Session holds per-document runtime validation state.
type Session struct {
	SessionIO
	SessionBuffers
	SessionIdentity
	AttributeTracker

	Names            names.State
	rt               *runtime.Schema
	Scratch          Scratch
	elemStack        []elemFrame
	validationErrors []xsderrors.Validation
	Arena            Arena
	normDepth        int
}
