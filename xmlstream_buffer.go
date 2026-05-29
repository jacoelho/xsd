package xsd

import "io"

type byteStream struct {
	r       io.Reader
	err     error
	lastPos bytePosition
	off     int
	end     int
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
		b.err = err
		return nil
	}
	if err != nil {
		return err
	}
	return io.ErrNoProgress
}

func (b *byteStream) consumeBuffered(n int) {
	if n <= 0 {
		return
	}
	if b.unread {
		b.unread = false
		b.advance(b.last)
		return
	}
	b.off += n
	b.col += n
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
