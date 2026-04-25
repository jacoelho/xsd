package xmlstream

import (
	"bufio"
	"errors"
	"io"

	"github.com/jacoelho/xsd/internal/xmltext"
)

const readerBufferSize = 256 * 1024
const readerAttrCapacity = 8

var errNilReader = errors.New("nil XML reader")
var errDuplicateAttribute = errors.New("duplicate attribute")

// Reader provides a streaming XML event interface with namespace tracking.
type Reader struct {
	nsBytes       namespaceBytesCache
	reader        *bufio.Reader
	resolvedNames *resolvedNameCache
	dec           *xmltext.Decoder
	names         *qnameCache
	ns            nsStack
	attrSeen      []uint32
	rawAttrInfo   []rawAttrInfo
	rawAttrBuf    []RawAttr
	resolvedAttr  []ResolvedAttr
	attrBuf       []Attr
	lastRawInfo   []rawAttrInfo
	elemStack     []elementStackEntry
	valueBuf      []byte
	nsBuf         []byte
	lastRawAttrs  []RawAttr
	tok           xmltext.RawTokenSpan
	lastStart     Event
	nextID        ElementID
	lastLine      int
	lastColumn    int
	attrEpoch     uint32
	pendingPop    bool
	lastWasStart  bool
}

type rawAttrInfo struct {
	namespace string
	local     []byte
}

// NewReader creates a new streaming reader for r.
func NewReader(r io.Reader, opts ...Option) (*Reader, error) {
	if r == nil {
		return nil, errNilReader
	}
	reader := bufio.NewReaderSize(r, readerBufferSize)
	options := buildOptions(opts...)
	dec := xmltext.NewDecoder(reader, options...)
	var tok xmltext.RawTokenSpan
	names := newQNameCache()
	names.setMaxEntries(qnameCacheLimit(options))
	return &Reader{
		reader:        reader,
		dec:           dec,
		names:         names,
		resolvedNames: newResolvedNameCache(),
		valueBuf:      make([]byte, 0, 256),
		nsBuf:         make([]byte, 0, 128),
		tok:           tok,
	}, nil
}

// Reset prepares the reader for a new input stream.
func (r *Reader) Reset(src io.Reader, opts ...Option) error {
	if r == nil {
		return errNilReader
	}
	if src == nil {
		return errNilReader
	}
	if r.reader == nil {
		r.reader = bufio.NewReaderSize(src, readerBufferSize)
	} else {
		r.reader.Reset(src)
	}
	options := buildOptions(opts...)
	if r.dec == nil {
		r.dec = xmltext.NewDecoder(r.reader, options...)
	} else {
		r.dec.Reset(r.reader, options...)
	}
	if r.names == nil {
		r.names = newQNameCache()
	} else {
		r.names.reset()
	}
	r.names.setMaxEntries(qnameCacheLimit(options))
	if r.resolvedNames == nil {
		r.resolvedNames = newResolvedNameCache()
	} else {
		r.resolvedNames.reset()
	}
	r.nsBytes.reset()
	r.ns.reset()
	r.attrBuf = r.attrBuf[:0]
	r.resolvedAttr = r.resolvedAttr[:0]
	r.rawAttrBuf = r.rawAttrBuf[:0]
	r.rawAttrInfo = r.rawAttrInfo[:0]
	r.attrSeen = r.attrSeen[:0]
	r.attrEpoch = 0
	r.elemStack = r.elemStack[:0]
	r.valueBuf = r.valueBuf[:0]
	r.nsBuf = r.nsBuf[:0]
	r.nextID = 0
	r.pendingPop = false
	r.lastLine = 0
	r.lastColumn = 0
	r.lastWasStart = false
	r.lastStart = Event{}
	r.lastRawAttrs = nil
	r.lastRawInfo = nil
	return nil
}

// Next returns the next XML event.
func (r *Reader) Next() (Event, error) {
	var ev Event
	err := r.next(nextEvent, &ev, nil, nil)
	return ev, err
}

// NextResolved returns the next XML event with namespace-resolved byte slices.
func (r *Reader) NextResolved() (ResolvedEvent, error) {
	var ev ResolvedEvent
	err := r.next(nextResolved, nil, nil, &ev)
	return ev, err
}

// NextRaw returns the next XML event with raw names.
// Raw name and value slices are valid until the next Next or NextRaw call.
func (r *Reader) NextRaw() (RawEvent, error) {
	var ev RawEvent
	err := r.next(nextRaw, nil, &ev, nil)
	return ev, err
}

// SkipSubtree skips the current element subtree after a StartElement event.
func (r *Reader) SkipSubtree() error {
	if r == nil || r.dec == nil {
		return errNilReader
	}
	if !r.lastWasStart {
		return errNoStartElement
	}
	if r.pendingPop {
		r.ns.pop()
		r.pendingPop = false
	}
	if err := r.dec.SkipValue(); err != nil {
		return err
	}
	if len(r.elemStack) > 0 {
		r.elemStack = r.elemStack[:len(r.elemStack)-1]
	}
	r.ns.pop()
	r.lastWasStart = false
	r.lastRawAttrs = nil
	r.lastRawInfo = nil
	return nil
}

// CurrentPos returns the line and column of the most recent token.
func (r *Reader) CurrentPos() (line, column int) {
	if r == nil {
		return 0, 0
	}
	return r.lastLine, r.lastColumn
}

// InputOffset returns the current byte position in the input stream.
func (r *Reader) InputOffset() int64 {
	if r == nil || r.dec == nil {
		return 0
	}
	return r.dec.InputOffset()
}
