package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (e *validationExecutor) processStartElement(ev *xmlstream.ResolvedEvent, stream subtreeSkipper) error {
	if err := e.s.handleStartElement(ev, e.resolver); err != nil {
		if fatal := e.s.recordValidationError(err, ev.Line, ev.Column); fatal != nil {
			return fatal
		}
		if skipErr := stream.SkipSubtree(); skipErr != nil {
			return xsderrors.ValidationList{e.s.newValidation(xsderrors.ErrXMLParse, skipErr.Error(), e.s.pathString(), 0, 0)}
		}
	}
	e.rootSeen = true
	e.allowBOM = false
	return nil
}
