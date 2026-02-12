package validator

import (
	"errors"
	"io"

	xsderrors "github.com/jacoelho/xsd/errors"
)

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
	if err := s.ensureReader(r); err != nil {
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
