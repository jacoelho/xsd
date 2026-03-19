package validator

import (
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (e *validationExecutor) processEndElement(ev *xmlstream.ResolvedEvent) error {
	errs, path := e.s.handleEndElement(ev, e.resolver)
	if len(errs) > 0 {
		if fatal := e.s.recordValidationErrorsAtPath(errs, path, ev.Line, ev.Column); fatal != nil {
			return fatal
		}
	}
	if e.s.icState.HasCommitted() {
		if pending := diag.AppendIssues(nil, e.s.icState.DrainCommitted()); len(pending) > 0 {
			if fatal := e.s.recordValidationErrorsAtPath(pending, path, ev.Line, ev.Column); fatal != nil {
				return fatal
			}
		}
	}
	e.allowBOM = false
	return nil
}
