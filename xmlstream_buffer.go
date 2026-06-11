package xsd

import (
	"bytes"
	"io"
)

// byteStream tracks line and column positions in bytes, not runes; columns
// inside multibyte UTF-8 sequences report byte offsets.
//
// nlIndex caches the index of the first '\n' at or after the position of the
// last newline scan: -1 marks the buffer window as unscanned, end marks a
// window with no remaining newline. consumeBuffered compares against it so
// chunks without line breaks skip scanning entirely.
type byteStream struct {
	r       io.Reader
	err     error
	lastPos bytePosition
	off     int
	end     int
	nlIndex int
	line    int
	col     int
	buf     [64 * 1024]byte
	unread  bool
	last    byte
}

func (b *byteStream) reset(r io.Reader) {
	b.r = r
	b.err = nil
	b.lastPos = bytePosition{}
	b.off = 0
	b.end = 0
	b.nlIndex = -1
	b.line = 1
	b.col = 0
	b.unread = false
	b.last = 0
}

type bytePosition struct {
	line int
	col  int
}

func (b *byteStream) readByte() (byte, error) {
	if b.unread {
		b.unread = false
		b.advance(b.last)
		return b.last, nil
	}
	if b.off == b.end {
		if b.err != nil {
			err := b.err
			b.err = nil
			return 0, err
		}
		n, err := b.r.Read(b.buf[:])
		if n <= 0 {
			if err != nil {
				return 0, err
			}
			return 0, io.ErrNoProgress
		}
		b.off = 0
		b.end = n
		b.nlIndex = -1
		b.err = err
	}
	c := b.buf[b.off]
	b.off++
	b.last = c
	b.lastPos = bytePosition{line: b.line, col: b.col}
	b.advance(c)
	return c, nil
}

func (b *byteStream) buffered() ([]byte, error) {
	if b.unread {
		return []byte{b.last}, nil
	}
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
		err := b.err
		b.err = nil
		return err
	}
	n, err := b.r.Read(b.buf[:])
	if n > 0 {
		b.off = 0
		b.end = n
		b.nlIndex = -1
		b.err = err
		return nil
	}
	if err != nil {
		return err
	}
	return io.ErrNoProgress
}

// consumeBuffered advances past n bytes previously returned by buffered;
// callers pass n > 0. Consumed bytes are checked for line breaks so position
// tracking stays correct for any chunk content.
func (b *byteStream) consumeBuffered(n int) {
	if b.unread || b.nlIndex < b.off+n {
		b.consumeBufferedSlow(n)
		return
	}
	b.off += n
	b.col += n
}

func (b *byteStream) consumeBufferedSlow(n int) {
	if b.unread {
		b.unread = false
		b.advance(b.last)
		return
	}
	start := b.off
	b.off += n
	b.col += n
	b.fixupLineBreaks(start)
}

// fixupLineBreaks repairs line and column after consuming buf[start:b.off]
// when nlIndex does not rule out a newline in the chunk. It rescans from
// start when nlIndex is stale (unscanned window, or a newline consumed via
// readByte) and leaves nlIndex at the first newline at or after b.off.
func (b *byteStream) fixupLineBreaks(start int) {
	if b.nlIndex < start {
		b.nlIndex = b.nextNewline(start)
		if b.nlIndex >= b.off {
			return
		}
	}
	last := b.nlIndex
	lines := 1
	for {
		i := bytes.IndexByte(b.buf[last+1:b.off], '\n')
		if i < 0 {
			break
		}
		last += 1 + i
		lines++
	}
	b.line += lines
	b.col = b.off - last - 1
	b.nlIndex = b.nextNewline(b.off)
}

func (b *byteStream) nextNewline(from int) int {
	if i := bytes.IndexByte(b.buf[from:b.end], '\n'); i >= 0 {
		return from + i
	}
	return b.end
}

func (b *byteStream) unreadByte() {
	if b.unread {
		panic("double unread")
	}
	b.unread = true
	b.line = b.lastPos.line
	b.col = b.lastPos.col
}

func (b *byteStream) advance(c byte) {
	if c == '\n' {
		b.line++
		b.col = 0
		return
	}
	b.col++
}

func (b *byteStream) pos() (int, int) {
	return b.line, b.col
}

type byteStringCache struct {
	recent  [8]string
	buckets map[uint64][]int
	entries []byteStringEntry
	next    uint8
}

type byteStringEntry struct {
	text string
}

func newByteStringCache() byteStringCache {
	return byteStringCache{buckets: make(map[uint64][]int)}
}

func (c *byteStringCache) intern(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if s, ok := c.recentString(b); ok {
		return s
	}
	if c.buckets == nil {
		*c = newByteStringCache()
	}
	if len(b) > maxByteStringCacheLen {
		return string(b)
	}
	h := hashBytes(b)
	for _, idx := range c.buckets[h] {
		if stringBytesEqual(c.entries[idx].text, b) {
			s := c.entries[idx].text
			c.remember(s)
			return s
		}
	}
	s := string(b)
	if len(c.entries) >= maxByteStringCacheEntries {
		return s
	}
	idx := len(c.entries)
	c.entries = append(c.entries, byteStringEntry{text: s})
	c.buckets[h] = append(c.buckets[h], idx)
	c.remember(s)
	return s
}

func (c *byteStringCache) recentString(b []byte) (string, bool) {
	for _, s := range c.recent {
		if stringBytesEqual(s, b) {
			return s, true
		}
	}
	return "", false
}

func (c *byteStringCache) remember(s string) {
	c.recent[c.next%uint8(len(c.recent))] = s
	c.next++
}

func hashBytes(b []byte) uint64 {
	const offset = 14695981039346656037
	const prime = 1099511628211
	h := uint64(offset)
	for _, c := range b {
		h ^= uint64(c)
		h *= prime
	}
	return h
}
