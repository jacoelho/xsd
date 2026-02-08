package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/xsdxml"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

type validationExecutor struct {
	s        *Session
	resolver sessionResolver
	rootSeen bool
	allowBOM bool
}

type subtreeSkipper interface {
	SkipSubtree() error
}

func newValidationExecutor(s *Session) *validationExecutor {
	return &validationExecutor{
		s:        s,
		resolver: sessionResolver{s: s},
		allowBOM: true,
	}
}

func (e *validationExecutor) process(ev *xmlstream.ResolvedEvent, stream subtreeSkipper) error {
	if e == nil || e.s == nil || ev == nil {
		return nil
	}
	switch ev.Kind {
	case xmlstream.EventStartElement:
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
	case xmlstream.EventEndElement:
		errs, path := e.s.handleEndElement(ev, e.resolver)
		if len(errs) > 0 {
			if fatal := e.s.recordValidationErrorsAtPath(errs, path, ev.Line, ev.Column); fatal != nil {
				return fatal
			}
		}
		if e.s.icState.hasCommitted() {
			pending := e.s.icState.drainCommitted()
			if len(pending) > 0 {
				if fatal := e.s.recordValidationErrorsAtPath(pending, path, ev.Line, ev.Column); fatal != nil {
					return fatal
				}
			}
		}
		e.allowBOM = false
	case xmlstream.EventCharData:
		if len(e.s.elemStack) == 0 {
			if !xsdxml.IsIgnorableOutsideRoot(ev.Text, e.allowBOM) {
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
	}
	return nil
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
	if errs := e.s.finalizeIdentity(); len(errs) > 0 {
		if fatal := e.s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	return e.s.validationList()
}
