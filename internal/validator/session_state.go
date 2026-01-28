package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// NameID identifies a name entry within a single document.
type NameID uint32

type nameEntry struct {
	Sym      runtime.SymbolID
	NS       runtime.NamespaceID
	LocalOff uint32
	LocalLen uint32
	NSOff    uint32
	NSLen    uint32
}

type elemFrame struct {
	modelState       ModelState
	text             TextState
	model            runtime.ModelRef
	name             NameID
	elem             runtime.ElemID
	typ              runtime.TypeID
	content          runtime.ContentKind
	mixed            bool
	nilled           bool
	hasChildElements bool
}

type nsFrame struct {
	off uint32
	len uint32
}

type nsDecl struct {
	prefixOff uint32
	prefixLen uint32
	nsOff     uint32
	nsLen     uint32
}

// Session holds per-document runtime validation state.
type Session struct {
	rt               *runtime.Schema
	reader           *xmlstream.Reader
	idTable          map[string]struct{}
	attrPresent      []bool
	attrAppliedBuf   []AttrApplied
	nameMap          []nameEntry
	valueBuf         []byte
	attrBuf          []StartAttr
	attrValidatedBuf []StartAttr
	elemStack        []elemFrame
	normBuf          []byte
	errBuf           []byte
	nameLocal        []byte
	nameNS           []byte
	textBuf          []byte
	nsDecls          []nsDecl
	idRefs           []string
	nsStack          []nsFrame
	icState          identityState
}

// NewSession creates a new runtime validation session.
func NewSession(rt *runtime.Schema) *Session {
	return &Session{rt: rt}
}

// Reset clears per-document state while retaining buffer capacity.
func (s *Session) Reset() {
	if s == nil {
		return
	}
	s.elemStack = s.elemStack[:0]
	s.nsStack = s.nsStack[:0]
	s.nsDecls = s.nsDecls[:0]
	s.nameMap = s.nameMap[:0]
	s.nameLocal = s.nameLocal[:0]
	s.nameNS = s.nameNS[:0]
	s.textBuf = s.textBuf[:0]
	s.normBuf = s.normBuf[:0]
	s.errBuf = s.errBuf[:0]
	s.valueBuf = s.valueBuf[:0]
	s.attrBuf = s.attrBuf[:0]
	s.attrValidatedBuf = s.attrValidatedBuf[:0]
	s.attrPresent = s.attrPresent[:0]
	s.attrAppliedBuf = s.attrAppliedBuf[:0]
	s.icState.reset()
	if s.idTable != nil {
		clear(s.idTable)
	}
	s.idRefs = s.idRefs[:0]
}

func (s *Session) hasIdentityConstraints() bool {
	return s != nil && s.rt != nil && len(s.rt.ICs) > 1
}
