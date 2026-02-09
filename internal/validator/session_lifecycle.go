package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (t *AttributeTracker) Reset() {
	if t == nil {
		return
	}
	t.attrAppliedBuf = t.attrAppliedBuf[:0]
	t.attrBuf = t.attrBuf[:0]
	t.attrValidatedBuf = t.attrValidatedBuf[:0]
	t.attrPresent = t.attrPresent[:0]
	t.attrSeenTable = t.attrSeenTable[:0]
}

// NewSession creates a new runtime validation session.
func NewSession(rt *runtime.Schema, opts ...xmlstream.Option) *Session {
	sess := &Session{rt: rt}
	if len(opts) > 0 {
		sess.parseOptions = append([]xmlstream.Option(nil), opts...)
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
	s.nsStack.Reset()
	s.nsDecls = s.nsDecls[:0]
	s.prefixCache = s.prefixCache[:0]
	s.nameMap = s.nameMap[:0]
	s.nameMapSparse = nil
	s.nameLocal = s.nameLocal[:0]
	s.nameNS = s.nameNS[:0]
	s.textBuf = s.textBuf[:0]
	s.keyBuf = s.keyBuf[:0]
	s.keyTmp = s.keyTmp[:0]
	s.normBuf = s.normBuf[:0]
	s.normDepth = 0
	s.errBuf = s.errBuf[:0]
	s.validationErrors = s.validationErrors[:0]
	s.valueBuf = s.valueBuf[:0]
	s.valueScratch = s.valueScratch[:0]
	s.AttributeTracker.Reset()
	s.icState.reset()
	s.documentURI = ""
	if s.idTable != nil {
		if len(s.idTable) > maxSessionIDTableEntries {
			s.idTable = nil
		} else {
			clear(s.idTable)
		}
	}
	s.idRefs = s.idRefs[:0]
	if s.identityAttrBuckets != nil {
		if len(s.identityAttrBuckets) > maxSessionEntries {
			s.identityAttrBuckets = nil
		} else {
			clear(s.identityAttrBuckets)
		}
	}
	s.identityAttrNames = s.identityAttrNames[:0]
	s.shrinkBuffers()
}
