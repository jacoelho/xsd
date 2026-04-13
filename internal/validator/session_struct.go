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
	SessionIO
	SessionBuffers
	SessionIdentity
	AttributeTracker

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
		sess.parseOptions = slices.Clone(opts)
	}
	sess.readerFactory = xmlstream.NewReader
	sess.icState.arena = &sess.Arena
	return sess
}

// Reset clears per-document state while retaining buffer capacity.
func (s *Session) Reset() {
	if s == nil {
		return
	}
	s.Arena.Reset()
	s.Scratch.Reset()
	s.icState.arena = &s.Arena
	s.elemStack = s.elemStack[:0]
	s.Names.Reset()
	s.textBuf = s.textBuf[:0]
	s.keyBuf = s.keyBuf[:0]
	s.keyTmp = s.keyTmp[:0]
	s.normBuf = s.normBuf[:0]
	s.normDepth = 0
	s.metricsDepth = 0
	s.errBuf = s.errBuf[:0]
	s.validationErrors = s.validationErrors[:0]
	s.valueBuf = s.valueBuf[:0]
	s.valueScratch = s.valueScratch[:0]
	s.AttributeTracker.Reset()
	s.icState.Reset()
	s.documentURI = ""
	s.resetIDTable()
	s.idRefs = s.idRefs[:0]
	s.identityAttrs.Reset(maxSessionEntries)
	s.shrinkBuffers()
}

func (s *Session) resetIDTable() {
	if s == nil || s.idTable == nil {
		return
	}
	if len(s.idTable) > maxSessionIDTableEntries {
		s.idTable = nil
		return
	}
	clear(s.idTable)
}
