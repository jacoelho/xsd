package validator

import (
	"errors"
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/xmlstream"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

// Validate validates an XML document using the runtime schema.
func (s *Session) Validate(r io.Reader) error {
	return s.ValidateWithDocument(r, "")
}

// ValidateWithDocument validates an XML document with a known document URI.
func (s *Session) ValidateWithDocument(r io.Reader, document string) error {
	if s == nil || s.rt == nil {
		return schemaNotLoadedError()
	}
	if r == nil {
		return readerSetupError(errors.New("nil reader"), document)
	}
	s.Reset()
	s.io.documentURI = document
	if err := s.ensureReader(r); err != nil {
		return readerSetupError(err, s.io.documentURI)
	}

	executor := validationExecutor{
		s:        s,
		allowBOM: true,
	}
	for {
		ev, err := s.io.reader.NextResolved()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return xsderrors.ValidationList{s.newValidation(xsderrors.ErrXMLParse, err.Error(), s.pathString(), 0, 0)}
		}
		if err := executor.process(&ev); err != nil {
			return err
		}
	}

	return executor.finalize()
}

func (s *Session) handleCharData(ev *xmlstream.ResolvedEvent) error {
	if ev == nil {
		return fmt.Errorf("character data event missing")
	}
	if len(s.elemStack) == 0 {
		return nil
	}
	frame := &s.elemStack[len(s.elemStack)-1]
	return s.ConsumeText(&frame.text, frame.content, frame.mixed, frame.nilled, ev.Text)
}
