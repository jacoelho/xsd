package start

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/model"
)

// ChildInput contains the parent-state fields needed to resolve one child start.
type ChildInput struct {
	Model   runtime.ModelRef
	Content runtime.ContentKind
	Nilled  bool
}

// ChildResult reports the model match plus whether the caller should mark the
// parent as having reported a child-content error.
type ChildResult struct {
	Match              model.Match
	ChildErrorReported bool
}

// StepModelFunc advances one caller-owned content model with the incoming element.
type StepModelFunc func(runtime.ModelRef, runtime.SymbolID, runtime.NamespaceID, []byte) (model.Match, error)

// ResolveChild applies parent content constraints before delegating to the model stepper.
func ResolveChild(in ChildInput, sym runtime.SymbolID, nsID runtime.NamespaceID, ns []byte, step StepModelFunc) (ChildResult, error) {
	if in.Nilled {
		return ChildResult{ChildErrorReported: true}, diag.New(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
	}
	if in.Content == runtime.ContentSimple || in.Content == runtime.ContentEmpty {
		if in.Content == runtime.ContentSimple {
			return ChildResult{ChildErrorReported: true}, diag.New(xsderrors.ErrTextInElementOnly, "element not allowed in simple content")
		}
		return ChildResult{ChildErrorReported: true}, diag.New(xsderrors.ErrUnexpectedElement, "element not allowed in empty content")
	}
	if in.Model.Kind == runtime.ModelNone {
		return ChildResult{}, diag.New(xsderrors.ErrUnexpectedElement, "no content model match")
	}
	if step == nil {
		return ChildResult{}, fmt.Errorf("model stepper missing")
	}

	match, err := step(in.Model, sym, nsID, ns)
	if err != nil {
		return ChildResult{}, err
	}
	return ChildResult{Match: match}, nil
}
