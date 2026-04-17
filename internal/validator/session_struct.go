package validator

import (
	"io"
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// SessionIO owns reader state and parsing options for one validator session.
type SessionIO struct {
	reader        *xmlstream.Reader
	readerFactory func(io.Reader, ...xmlstream.Option) (*xmlstream.Reader, error)
	documentURI   string
	parseOptions  []xmlstream.Option
}

// Session holds per-document runtime validation state.
type Session struct {
	io       SessionIO
	buffers  SessionBuffers
	identity SessionIdentity
	attrs    AttributeTracker

	Names            NameState
	rt               *runtime.Schema
	Scratch          Scratch
	elemStack        []elemFrame
	validationErrors []xsderrors.Validation
	Arena            Arena
	normDepth        int
}

// NewSession creates a new runtime validation session.
func NewSession(rt *runtime.Schema, opts ...xmlstream.Option) *Session {
	sess := &Session{rt: rt}
	if len(opts) > 0 {
		sess.io.parseOptions = slices.Clone(opts)
	}
	sess.io.readerFactory = xmlstream.NewReader
	sess.identity.icState.arena = &sess.Arena
	return sess
}

// Reset clears per-document state while retaining buffer capacity.
func (s *Session) Reset() {
	if s == nil {
		return
	}
	s.Arena.Reset()
	s.Scratch.Reset()
	s.elemStack = s.elemStack[:0]
	s.Names.Reset()
	s.buffers.Reset()
	s.normDepth = 0
	s.validationErrors = s.validationErrors[:0]
	s.attrs.Reset()
	s.identity.Reset(&s.Arena, maxSessionEntries, maxSessionIDTableEntries)
	s.io.Reset()
	s.shrinkBuffers()
}

func (io *SessionIO) Reset() {
	if io == nil {
		return
	}
	io.documentURI = ""
}
