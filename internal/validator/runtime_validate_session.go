package validator

import (
	"errors"
	"io"

	xsderrors "github.com/jacoelho/xsd/errors"
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
	return s.ValidateWithDocument(r, "")
}

// ValidateWithDocument validates an XML document with a known document URI.
func (s *Session) ValidateWithDocument(r io.Reader, document string) error {
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

	executor := newValidationExecutor(s)
	for {
		ev, err := s.reader.NextResolved()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return xsderrors.ValidationList{s.newValidation(xsderrors.ErrXMLParse, err.Error(), s.pathString(), 0, 0)}
		}
		if err := executor.process(&ev, s.reader); err != nil {
			return err
		}
	}

	return executor.finalize()
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
