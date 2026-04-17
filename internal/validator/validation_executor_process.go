package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

type validationExecutor struct {
	s        *Session
	rootSeen bool
	allowBOM bool
}

func (e *validationExecutor) process(ev *xmlstream.ResolvedEvent) error {
	if e == nil || e.s == nil || ev == nil {
		return nil
	}
	switch ev.Kind {
	case xmlstream.EventStartElement:
		return e.processStartElement(ev)
	case xmlstream.EventEndElement:
		return e.processEndElement(ev)
	case xmlstream.EventCharData:
		return e.processCharData(ev)
	default:
		return nil
	}
}

func (e *validationExecutor) finalize() error {
	if e == nil || e.s == nil {
		return nil
	}
	if !e.rootSeen {
		return xsderrors.ValidationList{e.s.newValidation(xsderrors.ErrNoRoot, "document has no root element", "", 0, 0)}
	}
	if len(e.s.elemStack) != 0 {
		return xsderrors.ValidationList{e.s.newValidation(xsderrors.ErrXMLParse, "document ended with unclosed elements", e.s.pathString(), 0, 0)}
	}
	if errs := e.s.validateIDRefs(); len(errs) > 0 {
		if fatal := e.s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	if errs := e.s.identity.icState.DrainUncommitted(); len(errs) > 0 {
		if fatal := e.s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	if errs := xsderrors.AppendIssues(nil, e.s.identity.icState.DrainCommitted()); len(errs) > 0 {
		if fatal := e.s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	return e.s.validationList()
}

func (e *validationExecutor) processStartElement(ev *xmlstream.ResolvedEvent) error {
	if err := e.s.handleStartElement(ev, sessionResolver{s: e.s}); err != nil {
		if fatal := e.s.recordValidationError(err, ev.Line, ev.Column); fatal != nil {
			return fatal
		}
		if skipErr := e.s.io.reader.SkipSubtree(); skipErr != nil {
			return xsderrors.ValidationList{e.s.newValidation(xsderrors.ErrXMLParse, skipErr.Error(), e.s.pathString(), 0, 0)}
		}
	}
	e.rootSeen = true
	e.allowBOM = false
	return nil
}

func (e *validationExecutor) processEndElement(ev *xmlstream.ResolvedEvent) error {
	errs, path := e.s.handleEndElement(ev, sessionResolver{s: e.s})
	if len(errs) > 0 {
		if fatal := e.s.recordValidationErrorsAtPath(errs, path, ev.Line, ev.Column); fatal != nil {
			return fatal
		}
	}
	if e.s.identity.icState.HasCommitted() {
		if pending := xsderrors.AppendIssues(nil, e.s.identity.icState.DrainCommitted()); len(pending) > 0 {
			if fatal := e.s.recordValidationErrorsAtPath(pending, path, ev.Line, ev.Column); fatal != nil {
				return fatal
			}
		}
	}
	e.allowBOM = false
	return nil
}

func (e *validationExecutor) processCharData(ev *xmlstream.ResolvedEvent) error {
	if len(e.s.elemStack) == 0 {
		if !value.IsIgnorableOutsideRoot(ev.Text, e.allowBOM) {
			if fatal := e.s.recordValidationError(fmt.Errorf("unexpected character data outside root element"), ev.Line, ev.Column); fatal != nil {
				return fatal
			}
		}
		e.allowBOM = false
		return nil
	}
	if err := e.s.handleCharData(ev); err != nil {
		if fatal := e.s.recordValidationError(err, ev.Line, ev.Column); fatal != nil {
			return fatal
		}
	}
	e.allowBOM = false
	return nil
}
