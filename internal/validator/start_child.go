package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// StartChildInput contains the parent-state fields needed to resolve one child start.
type StartChildInput struct {
	Model   runtime.ModelRef
	Content runtime.ContentKind
	Nilled  bool
}

// StartChildResult reports the model match plus whether the caller should mark the
// parent as having reported a child-content error.
type StartChildResult struct {
	Match              StartMatch
	ChildErrorReported bool
}

// StartStepModelFunc advances one caller-owned content model with the incoming element.
type StartStepModelFunc func(runtime.ModelRef, runtime.SymbolID, runtime.NamespaceID, []byte) (StartMatch, error)

// ResolveStartChild applies parent content constraints before delegating to the model stepper.
func ResolveStartChild(in StartChildInput, sym runtime.SymbolID, nsID runtime.NamespaceID, ns []byte, step StartStepModelFunc) (StartChildResult, error) {
	if in.Nilled {
		return StartChildResult{ChildErrorReported: true}, xsderrors.New(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
	}
	if in.Content == runtime.ContentSimple || in.Content == runtime.ContentEmpty {
		if in.Content == runtime.ContentSimple {
			return StartChildResult{ChildErrorReported: true}, xsderrors.New(xsderrors.ErrTextInElementOnly, "element not allowed in simple content")
		}
		return StartChildResult{ChildErrorReported: true}, xsderrors.New(xsderrors.ErrUnexpectedElement, "element not allowed in empty content")
	}
	if in.Model.Kind == runtime.ModelNone {
		return StartChildResult{}, xsderrors.New(xsderrors.ErrUnexpectedElement, "no content model match")
	}
	if step == nil {
		return StartChildResult{}, fmt.Errorf("model stepper missing")
	}

	match, err := step(in.Model, sym, nsID, ns)
	if err != nil {
		return StartChildResult{}, err
	}
	return StartChildResult{Match: match}, nil
}
