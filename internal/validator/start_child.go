package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

// startChildInput contains the parent-state fields needed to resolve one child start.
type startChildInput struct {
	Model   runtime.ModelRef
	Content runtime.ContentKind
	Nilled  bool
}

// startChildResult reports the model match plus whether the caller should mark the
// parent as having reported a child-content error.
type startChildResult struct {
	Match              StartMatch
	ChildErrorReported bool
}

// startStepModelFunc advances one caller-owned content model with the incoming element.
type startStepModelFunc func(runtime.ModelRef, runtime.SymbolID, runtime.NamespaceID, []byte) (StartMatch, error)

// resolveStartChild applies parent content constraints before delegating to the model stepper.
func resolveStartChild(in startChildInput, sym runtime.SymbolID, nsID runtime.NamespaceID, ns []byte, step startStepModelFunc) (startChildResult, error) {
	if in.Nilled {
		return startChildResult{ChildErrorReported: true}, xsderrors.New(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
	}
	if in.Content == runtime.ContentSimple || in.Content == runtime.ContentEmpty {
		if in.Content == runtime.ContentSimple {
			return startChildResult{ChildErrorReported: true}, xsderrors.New(xsderrors.ErrTextInElementOnly, "element not allowed in simple content")
		}
		return startChildResult{ChildErrorReported: true}, xsderrors.New(xsderrors.ErrUnexpectedElement, "element not allowed in empty content")
	}
	if in.Model.Kind == runtime.ModelNone {
		return startChildResult{}, xsderrors.New(xsderrors.ErrUnexpectedElement, "no content model match")
	}
	if step == nil {
		return startChildResult{}, fmt.Errorf("model stepper missing")
	}

	match, err := step(in.Model, sym, nsID, ns)
	if err != nil {
		return startChildResult{}, err
	}
	return startChildResult{Match: match}, nil
}
