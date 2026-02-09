package validator

import (
	"io"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/state"
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
	local              []byte
	ns                 []byte
	modelState         ModelState
	text               TextState
	model              runtime.ModelRef
	name               NameID
	elem               runtime.ElemID
	typ                runtime.TypeID
	content            runtime.ContentKind
	mixed              bool
	nilled             bool
	hasChildElements   bool
	childErrorReported bool
}

type nsFrame struct {
	off      uint32
	len      uint32
	cacheOff uint32
}

type nsDecl struct {
	prefixOff  uint32
	prefixLen  uint32
	nsOff      uint32
	nsLen      uint32
	prefixHash uint64
}

type prefixEntry struct {
	hash      uint64
	prefixOff uint32
	prefixLen uint32
	nsOff     uint32
	nsLen     uint32
	ok        bool
}

type attrSeenEntry struct {
	hash uint64
	idx  uint32
}

type SessionIO struct {
	reader        *xmlstream.Reader
	readerFactory func(io.Reader, ...xmlstream.Option) (*xmlstream.Reader, error)
	documentURI   string
	parseOptions  []xmlstream.Option
}

type AttributeTracker struct {
	attrAppliedBuf   []AttrApplied
	attrPresent      []bool
	attrBuf          []StartAttr
	attrValidatedBuf []StartAttr
	attrSeenTable    []attrSeenEntry
}

type SessionBuffers struct {
	normBuf      []byte
	valueBuf     []byte
	valueScratch []byte
	normStack    [][]byte
	prefixCache  []prefixEntry
	nameLocal    []byte
	errBuf       []byte
	nameNS       []byte
	textBuf      []byte
	keyBuf       []byte
	keyTmp       []byte
	nsDecls      []nsDecl
}

type SessionIdentity struct {
	idTable             map[string]struct{}
	identityAttrBuckets map[uint64][]identityAttrNameID
	idRefs              []string
	identityAttrNames   []identityAttrName
	icState             identityState
}

type identityAttrName struct {
	ns    []byte
	local []byte
}

// Session holds per-document runtime validation state.
type Session struct {
	SessionIO
	SessionBuffers
	SessionIdentity
	AttributeTracker

	nameMapSparse    map[NameID]nameEntry
	rt               *runtime.Schema
	Scratch          Scratch
	nameMap          []nameEntry
	elemStack        []elemFrame
	validationErrors []xsderrors.Validation
	nsStack          state.StateStack[nsFrame]
	Arena            Arena
	normDepth        int
}
