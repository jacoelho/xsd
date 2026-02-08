package validator

import (
	"errors"
	"io"

	xsderrors "github.com/jacoelho/xsd/errors"
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

	stream := newStreamController(s)
	if err := stream.Reset(r); err != nil {
		return err
	}
	executor := newValidationExecutor(s)
	for {
		ev, err := stream.NextResolved()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return xsderrors.ValidationList{s.newValidation(xsderrors.ErrXMLParse, err.Error(), s.pathString(), 0, 0)}
		}
		if err := executor.process(&ev, stream); err != nil {
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
