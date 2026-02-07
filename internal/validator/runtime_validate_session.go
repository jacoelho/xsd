package validator

import (
	"errors"
	"fmt"
	"io"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/xsdxml"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

const maxNameMapSize = 1 << 20

type sessionResolver struct {
	s *Session
}

func (r sessionResolver) ResolvePrefix(prefix []byte) ([]byte, bool) {
	if r.s == nil {
		return nil, false
	}
	return r.s.lookupNamespace(prefix)
}

// Validate validates an XML document using the runtime schema.
func (s *Session) Validate(r io.Reader) error {
	return s.validateWithDocument(r, "")
}

// ValidateWithDocument validates an XML document with a known document URI.
func (s *Session) ValidateWithDocument(r io.Reader, document string) error {
	return s.validateWithDocument(r, document)
}

func (s *Session) validateWithDocument(r io.Reader, document string) error {
	if s == nil || s.rt == nil {
		return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrSchemaNotLoaded, "schema not loaded", "")}
	}
	if r == nil {
		return readerSetupError(errors.New("nil reader"), document)
	}
	s.Reset()
	s.documentURI = document

	if s.reader == nil {
		factory := s.readerFactory
		if factory == nil {
			factory = xmlstream.NewReader
		}
		reader, err := factory(r, s.parseOptions...)
		if err != nil {
			return readerSetupError(err, s.documentURI)
		}
		s.reader = reader
	} else if err := s.reader.Reset(r, s.parseOptions...); err != nil {
		return readerSetupError(err, s.documentURI)
	}

	rootSeen := false
	allowBOM := true
	resolver := sessionResolver{s: s}
	for {
		ev, err := s.reader.NextResolved()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return xsderrors.ValidationList{s.newValidation(xsderrors.ErrXMLParse, err.Error(), s.pathString(), 0, 0)}
		}
		if err := s.processResolvedEvent(&ev, resolver, &rootSeen, &allowBOM); err != nil {
			return err
		}
	}

	if !rootSeen {
		return xsderrors.ValidationList{s.newValidation(xsderrors.ErrNoRoot, "document has no root element", "", 0, 0)}
	}
	if len(s.elemStack) != 0 {
		return xsderrors.ValidationList{s.newValidation(xsderrors.ErrXMLParse, "document ended with unclosed elements", s.pathString(), 0, 0)}
	}
	if errs := s.validateIDRefs(); len(errs) > 0 {
		if fatal := s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	if errs := s.finalizeIdentity(); len(errs) > 0 {
		if fatal := s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	return s.validationList()
}

func (s *Session) processResolvedEvent(ev *xmlstream.ResolvedEvent, resolver sessionResolver, rootSeen, allowBOM *bool) error {
	switch ev.Kind {
	case xmlstream.EventStartElement:
		if err := s.handleStartElement(ev, resolver); err != nil {
			if fatal := s.recordValidationError(err, ev.Line, ev.Column); fatal != nil {
				return fatal
			}
			if skipErr := s.reader.SkipSubtree(); skipErr != nil {
				return xsderrors.ValidationList{s.newValidation(xsderrors.ErrXMLParse, skipErr.Error(), s.pathString(), 0, 0)}
			}
		}
		if !*rootSeen {
			*rootSeen = true
		}
		*allowBOM = false
	case xmlstream.EventEndElement:
		errs, path := s.handleEndElement(ev, resolver)
		if len(errs) > 0 {
			if fatal := s.recordValidationErrorsAtPath(errs, path, ev.Line, ev.Column); fatal != nil {
				return fatal
			}
		}
		if len(s.icState.pending) > 0 {
			pending := s.icState.drainPending()
			if len(pending) > 0 {
				if fatal := s.recordValidationErrorsAtPath(pending, path, ev.Line, ev.Column); fatal != nil {
					return fatal
				}
			}
		}
		*allowBOM = false
	case xmlstream.EventCharData:
		if len(s.elemStack) == 0 {
			if !xsdxml.IsIgnorableOutsideRoot(ev.Text, *allowBOM) {
				if fatal := s.recordValidationError(fmt.Errorf("unexpected character data outside root element"), ev.Line, ev.Column); fatal != nil {
					return fatal
				}
			}
			*allowBOM = false
			return nil
		}
		if err := s.handleCharData(ev); err != nil {
			if fatal := s.recordValidationError(err, ev.Line, ev.Column); fatal != nil {
				return fatal
			}
		}
		*allowBOM = false
	}
	return nil
}

func readerSetupError(err error, document string) error {
	if err == nil {
		return nil
	}
	return xsderrors.ValidationList{{
		Code:     string(xsderrors.ErrXMLParse),
		Message:  err.Error(),
		Document: document,
	}}
}
