package xmltext

// Span identifies a half-open byte range within a decoder buffer.
type Span struct {
	buf   *spanBuffer
	Start int
	End   int
	gen   uint32
}

// QNameSpan holds span offsets for a qualified name.
type QNameSpan struct {
	Full      Span
	Prefix    Span
	Local     Span
	HasPrefix bool
}

// AttrSpan holds span offsets for a parsed attribute.
type AttrSpan struct {
	ValueSpan Span
	Name      QNameSpan
}

type spanBuffer struct {
	entities *entityResolver
	data     []byte
	gen      uint32
	poison   bool
	stable   bool
}

func makeSpan(buf *spanBuffer, start, end int) Span {
	if buf == nil {
		return Span{Start: start, End: end}
	}
	return Span{Start: start, End: end, buf: buf, gen: buf.gen}
}

func (s Span) bytes() []byte {
	if s.buf == nil {
		return nil
	}
	if s.buf.poison && s.gen != s.buf.gen {
		panic("xmltext: span is invalid after buffer reuse")
	}
	if s.Start < 0 || s.End < s.Start || s.End > len(s.buf.data) {
		if s.buf.poison {
			panic("xmltext: span bounds are invalid")
		}
		return nil
	}
	return s.buf.data[s.Start:s.End]
}

func (s Span) bytesUnsafe() []byte {
	if s.buf == nil {
		return nil
	}
	return s.buf.data[s.Start:s.End]
}
