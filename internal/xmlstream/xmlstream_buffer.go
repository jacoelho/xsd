package xmlstream

import (
	"encoding/binary"
	"errors"
	"io"
)

// byteStream tracks line and column positions in bytes, not runes; columns
// inside multibyte UTF-8 sequences report byte offsets. It is the only owner of
// logical line positions: a literal CR or CRLF advances one line in every
// parser mode, while each token mode normalizes its own payload (LF in text,
// comments, and PIs; a space in attribute values).
type byteStream struct {
	r   io.Reader
	err error
	// buf grows on demand so small inputs such as schema documents do not
	// allocate and clear the full window.
	buf      []byte
	off      int
	end      int
	line     int
	col      int
	maxBytes int64
	// admittedBytes excludes the single overflow probe so offsets refer only
	// to bytes exposed to tokenization.
	admittedBytes int64
	afterCR       bool
}

const (
	xmlInitialInputBufferSize   = 4 * 1024
	xmlInputBufferSize          = 64 * 1024
	maxConsecutiveEmptyXMLReads = 100
)

func (b *byteStream) reset(r io.Reader, maxBytes int64) {
	b.r = r
	b.err = nil
	b.off = 0
	b.end = 0
	b.line = 1
	b.col = 0
	b.maxBytes = maxBytes
	b.admittedBytes = 0
	b.afterCR = false
}

func (b *byteStream) detach() {
	b.r = nil
	b.err = nil
	b.off = 0
	b.end = 0
	b.afterCR = false
	b.maxBytes = 0
	b.admittedBytes = 0
}

// read admits at most maxBytes raw input bytes to the parser. It may consume
// one additional byte from the caller to prove that the limit was exceeded,
// but that byte is never exposed to tokenization. When the boundary-crossing
// read also fails, both causes are retained.
func (b *byteStream) read(p []byte) (int, error) {
	if b.maxBytes > 0 {
		remaining := b.maxBytes - b.admittedBytes
		if remaining < 0 {
			return 0, errXMLInputLimit
		}
		if int64(len(p)) > remaining {
			p = p[:remaining+1]
		}
	}
	n, err := b.readUnderlying(p)
	if n <= 0 {
		return n, err
	}
	if b.maxBytes <= 0 {
		b.admittedBytes += int64(n)
		return n, err
	}
	remaining := b.maxBytes - b.admittedBytes
	if int64(n) <= remaining {
		b.admittedBytes += int64(n)
		return n, err
	}
	admitted := int(remaining)
	b.admittedBytes += remaining
	limitErr := errXMLInputLimit
	if err != nil {
		limitErr = errors.Join(limitErr, err)
	}
	return admitted, limitErr
}

func (b *byteStream) readUnderlying(p []byte) (int, error) {
	for range maxConsecutiveEmptyXMLReads {
		n, err := b.r.Read(p)
		// End of input is io.EOF itself, as for io.ReadAll and
		// internal/source; a wrapped or joined EOF is a reader failure.
		if err != nil && err != io.EOF {
			err = &inputError{cause: err}
		}
		if n != 0 || err != nil {
			return n, err
		}
	}
	return 0, &inputError{cause: io.ErrNoProgress}
}

// ensure returns the non-consuming input window after reading until at least n
// bytes are available, an error is known, or the fixed input buffer is full.
// An error returned with bytes is deferred when the window already satisfies n.
func (b *byteStream) ensure(n int) ([]byte, error) {
	for b.end-b.off < n && b.hasRoom() {
		if b.err != nil {
			return b.buf[b.off:b.end], b.err
		}
		if b.r == nil {
			return b.buf[b.off:b.end], errNilReader()
		}
		read, err := b.read(b.buf[b.end:])
		if read > 0 {
			b.end += read
			b.err = err
			continue
		}
		if err != nil {
			b.err = err
			return b.buf[b.off:b.end], err
		}
		return b.buf[b.off:b.end], &inputError{cause: io.ErrNoProgress}
	}
	return b.buf[b.off:b.end], nil
}

func (b *byteStream) hasRoom() bool {
	return b.end < len(b.buf) || b.growWindow()
}

func (b *byteStream) growWindow() bool {
	if len(b.buf) >= xmlInputBufferSize {
		return false
	}
	buf := make([]byte, min(max(xmlInitialInputBufferSize, 2*len(b.buf)), xmlInputBufferSize))
	copy(buf, b.buf[:b.end])
	b.buf = buf
	return true
}

func (b *byteStream) discardUTF8BOM() {
	copy(b.buf, b.buf[utf8BOMLen:b.end])
	b.end -= utf8BOMLen
	b.off = 0
}

func (b *byteStream) readByte() (byte, error) {
	if b.off == b.end {
		if err := b.fill(); err != nil {
			return 0, err
		}
	}
	c := b.buf[b.off]
	b.off++
	b.advance(c)
	return c, nil
}

// peekByte returns the next byte without consuming it or changing the
// logical source position. Keeping lookahead in the input window avoids
// restoring a byte's line/column snapshot after a speculative read.
func (b *byteStream) peekByte() (byte, error) {
	if b.off == b.end {
		if err := b.fill(); err != nil {
			return 0, err
		}
	}
	return b.buf[b.off], nil
}

func (b *byteStream) buffered() ([]byte, error) {
	if err := b.fill(); err != nil {
		return nil, err
	}
	return b.buf[b.off:b.end], nil
}

func (b *byteStream) fill() error {
	if b.off != b.end {
		return nil
	}
	if b.err != nil {
		// Keep the stored error terminal. In particular, a limit probe leaves
		// admittedBytes at maxBytes, so retrying here would issue another read.
		return b.err
	}
	if b.r == nil {
		return errNilReader()
	}
	if b.end == len(b.buf) {
		b.growWindow()
	}
	n, err := b.read(b.buf)
	if n > 0 {
		b.off = 0
		b.end = n
		b.err = err
		return nil
	}
	if err != nil {
		b.err = err
		return err
	}
	b.err = &inputError{cause: io.ErrNoProgress}
	return b.err
}

// consumeBuffered advances past n bytes previously returned by buffered.
// Callers pass n > 0 after proving the bytes contain neither CR nor LF.
func (b *byteStream) consumeBuffered(n int) {
	b.off += n
	b.col += n
	b.afterCR = false
}

// consumeBufferedLF advances over a run of literal LF bytes. XML line
// normalization makes each LF a logical line boundary, so the run can update
// positions in one operation while retaining the same source offsets.
func (b *byteStream) consumeBufferedLF(n int) {
	consumed := n
	if b.afterCR && n > 0 {
		// The first LF completes a preceding CRLF pair and therefore does not
		// start another logical line. The remaining literal LF bytes do.
		b.afterCR = false
		n--
	}
	b.off += consumed
	b.line += n
	b.col = 0
	b.afterCR = false
}

func (b *byteStream) advance(c byte) {
	// One range check keeps ordinary token bytes on the parser's hot path.
	if c-'\n' <= '\r'-'\n' {
		switch c {
		case '\r':
			b.line++
			b.col = 0
			b.afterCR = true
			return
		case '\n':
			if b.afterCR {
				b.afterCR = false
				return
			}
			b.line++
			b.col = 0
			return
		}
	}
	if b.afterCR {
		b.afterCR = false
	}
	b.col++
}

func (b *byteStream) pos() (line, column int) {
	return b.line, b.col
}

// offset reports the logical source position of the next byte.
func (b *byteStream) offset() int {
	offset := b.admittedBytes - int64(b.end-b.off)
	if offset < 0 {
		return 0
	}
	return int(offset)
}

// Cache interns short parser strings per validation session. Each cache keeps
// at most maxByteStringCacheEntries strings no longer than
// maxByteStringCacheLen bytes, and survives Reader.Reset within that bound.
// The name cache holds only validated prefix and local NCName components,
// never expanded names, so every start admission resolves namespace bindings
// anew.
type cache struct {
	state *cacheState
}

const (
	// The ring uses masking and must remain a power of two.
	recentRingEntries     = 8
	recentRingMask        = recentRingEntries - 1
	recentCacheSetBits    = 5
	recentCacheSets       = 1 << recentCacheSetBits
	recentCachePromotions = 1024
)

// Each map key and value share one owned spelling. The value lets a lookup
// using borrowed bytes return the owned string without allocating a copy.
//
// Recent spellings sit in front of the map in one of two forms. A cache starts
// with an 8-entry ring, the cheapest lookup for the few names of a schema
// document or a short instance. After recentCachePromotions ring misses it
// moves them to a two-way table of recentCacheSets sets indexed by a hash of
// the name bytes, which keeps hitting on vocabularies wider than the ring
// whatever order they repeat in.
type cacheState struct {
	interned map[string]string
	table    *[recentCacheSets][2]string
	ring     [recentRingEntries]string
	misses   int
	next     uint8
}

// Intern returns a cached string copy of b when b is small enough to cache.
func (c *cache) Intern(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	state := c.cacheState()
	if state.table != nil {
		return state.internHashed(b)
	}
	if s, ok := state.ringString(b); ok {
		return s
	}
	if len(b) > maxByteStringCacheLen {
		return string(b)
	}
	state.misses++
	if state.misses >= recentCachePromotions {
		state.promote()
		return state.internHashed(b)
	}
	s, ok := state.lookupOrInsert(b)
	if ok {
		state.remember(s)
	}
	return s
}

func (c *cache) cacheState() *cacheState {
	if c.state == nil {
		c.state = &cacheState{interned: make(map[string]string)}
	}
	return c.state
}

func (c *cacheState) internHashed(b []byte) string {
	if len(b) > maxByteStringCacheLen {
		return string(b)
	}
	set := &c.table[nameHash(b)>>(64-recentCacheSetBits)]
	if stringBytesEqual(set[0], b) {
		return set[0]
	}
	if stringBytesEqual(set[1], b) {
		return set[1]
	}
	s, ok := c.lookupOrInsert(b)
	if ok {
		set[1], set[0] = set[0], s
	}
	return s
}

// lookupOrInsert returns the owned spelling of b and whether the map holds it.
// A full map returns an unowned copy.
func (c *cacheState) lookupOrInsert(b []byte) (string, bool) {
	if s, ok := c.interned[string(b)]; ok {
		return s, true
	}
	s := string(b)
	if len(c.interned) >= maxByteStringCacheEntries {
		return s, false
	}
	c.interned[s] = s
	return s, true
}

func (c *cacheState) ringString(b []byte) (string, bool) {
	for offset := range recentRingEntries {
		index := (int(c.next) + recentRingEntries - 1 - offset) & recentRingMask
		s := c.ring[index]
		if stringBytesEqual(s, b) {
			return s, true
		}
	}
	return "", false
}

func (c *cacheState) remember(s string) {
	c.ring[c.next&recentRingMask] = s
	c.next++
}

func (c *cacheState) promote() {
	c.table = new([recentCacheSets][2]string)
	for offset := range recentRingEntries {
		s := c.ring[(int(c.next)+offset)&recentRingMask]
		if s == "" {
			continue
		}
		set := &c.table[nameHash([]byte(s))>>(64-recentCacheSetBits)]
		set[1], set[0] = set[0], s
	}
	c.ring = [recentRingEntries]string{}
}

func nameHash(b []byte) uint64 {
	n := len(b)
	h := uint64(n) * 0x9e3779b97f4a7c15
	switch {
	case n > 64:
		for i := 0; i < 32; i += 8 {
			h = mixNameWord(h, binary.LittleEndian.Uint64(b[i:]))
			h = mixNameWord(h, binary.LittleEndian.Uint64(b[n-32+i:]))
		}
	case n >= 8:
		for i := 0; i+8 < n; i += 8 {
			h = mixNameWord(h, binary.LittleEndian.Uint64(b[i:]))
		}
		h = mixNameWord(h, binary.LittleEndian.Uint64(b[n-8:]))
	case n >= 4:
		h = mixNameWord(h, uint64(binary.LittleEndian.Uint32(b))|uint64(binary.LittleEndian.Uint32(b[n-4:]))<<32)
	default:
		var w uint64
		for i, c := range b {
			w |= uint64(c) << (8 * i)
		}
		h = mixNameWord(h, w)
	}
	return h
}

func mixNameWord(h, w uint64) uint64 {
	h = (h ^ w) * 0xbf58476d1ce4e5b9
	return h ^ h>>31
}
