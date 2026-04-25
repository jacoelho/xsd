package validator

import (
	"io"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xmlstream"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
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
	metrics  ValueMetrics
	identity SessionIdentity
	attrs    AttributeTracker

	Names            NameState
	rt               *runtime.Schema
	plan             runtime.SessionPlan
	Scratch          Scratch
	elemStack        []elemFrame
	validationErrors []xsderrors.Validation
	Arena            Arena
	normDepth        int
}

// NewSession creates a new runtime validation session.
func NewSession(rt *runtime.Schema, opts ...xmlstream.Option) *Session {
	return NewSessionWithPlan(rt, runtime.NewSessionPlan(rt), opts...)
}

// NewSessionWithPlan creates a validation session with caller-supplied buffer sizing hints.
func NewSessionWithPlan(rt *runtime.Schema, plan runtime.SessionPlan, opts ...xmlstream.Option) *Session {
	sess := &Session{rt: rt, plan: plan}
	if len(opts) > 0 {
		sess.io.parseOptions = slices.Clone(opts)
	}
	sess.io.readerFactory = xmlstream.NewReader
	sess.identity.icState.arena = &sess.Arena
	sess.applySessionPlan()
	return sess
}

// Reset clears per-document state while retaining buffer capacity.
func (s *Session) Reset() {
	if s == nil {
		return
	}
	s.Arena.Reset()
	s.Scratch.Reset()
	s.metrics = ValueMetrics{}
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
