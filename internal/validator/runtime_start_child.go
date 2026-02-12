package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) resolveChildStartMatch(parent *elemFrame, sym runtime.SymbolID, nsID runtime.NamespaceID, ns []byte) (StartMatch, error) {
	if parent == nil {
		return StartMatch{}, fmt.Errorf("parent frame missing")
	}
	if parent.nilled {
		parent.childErrorReported = true
		return StartMatch{}, newValidationError(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
	}
	if parent.content == runtime.ContentSimple || parent.content == runtime.ContentEmpty {
		parent.childErrorReported = true
		if parent.content == runtime.ContentSimple {
			return StartMatch{}, newValidationError(xsderrors.ErrTextInElementOnly, "element not allowed in simple content")
		}
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "element not allowed in empty content")
	}
	if parent.model.Kind == runtime.ModelNone {
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
	}
	return s.StepModel(parent.model, &parent.modelState, sym, nsID, ns)
}
