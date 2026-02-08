package validator

import (
	"errors"
	"fmt"
	"io"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

type streamController struct {
	s *Session
}

func newStreamController(s *Session) *streamController {
	return &streamController{s: s}
}

func (c *streamController) Reset(r io.Reader) error {
	if c == nil || c.s == nil {
		return readerSetupError(errors.New("nil session"), "")
	}
	if r == nil {
		return readerSetupError(errors.New("nil reader"), c.s.documentURI)
	}
	if c.s.reader == nil {
		factory := c.s.readerFactory
		if factory == nil {
			factory = xmlstream.NewReader
		}
		reader, err := factory(r, c.s.parseOptions...)
		if err != nil {
			return readerSetupError(err, c.s.documentURI)
		}
		c.s.reader = reader
		return nil
	}
	if err := c.s.reader.Reset(r, c.s.parseOptions...); err != nil {
		return readerSetupError(err, c.s.documentURI)
	}
	return nil
}

func (c *streamController) NextResolved() (xmlstream.ResolvedEvent, error) {
	if c == nil || c.s == nil || c.s.reader == nil {
		return xmlstream.ResolvedEvent{}, xsderrors.ValidationList{{
			Code:     string(xsderrors.ErrXMLParse),
			Message:  "xml reader is not initialized",
			Document: c.documentURI(),
		}}
	}
	return c.s.reader.NextResolved()
}

func (c *streamController) SkipSubtree() error {
	if c == nil || c.s == nil || c.s.reader == nil {
		return fmt.Errorf("xml reader is not initialized")
	}
	return c.s.reader.SkipSubtree()
}

func (c *streamController) documentURI() string {
	if c == nil || c.s == nil {
		return ""
	}
	return c.s.documentURI
}
