package validator

import (
	"io"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) ensureReader(r io.Reader) error {
	if s == nil {
		return nil
	}
	if s.reader == nil {
		factory := s.readerFactory
		if factory == nil {
			factory = xmlstream.NewReader
		}
		reader, err := factory(r, s.parseOptions...)
		if err != nil {
			return err
		}
		s.reader = reader
		return nil
	}
	return s.reader.Reset(r, s.parseOptions...)
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
