package validator

import (
	"io"

	"github.com/jacoelho/xsd/internal/xmlstream"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func (s *Session) ensureReader(r io.Reader) error {
	if s == nil {
		return nil
	}
	if s.io.reader == nil {
		factory := s.io.readerFactory
		if factory == nil {
			factory = xmlstream.NewReader
		}
		reader, err := factory(r, s.io.parseOptions...)
		if err != nil {
			return err
		}
		s.io.reader = reader
		return nil
	}
	return s.io.reader.Reset(r, s.io.parseOptions...)
}

func readerSetupError(err error, document string) error {
	if err == nil {
		return nil
	}
	return xsderrors.ValidationList{{
		Code:     xsderrors.ErrXMLParse,
		Message:  err.Error(),
		Document: document,
	}}
}

func schemaNotLoadedError() error {
	return xsderrors.NewKind(xsderrors.KindCaller, xsderrors.ErrSchemaNotLoaded, "schema not loaded")
}
