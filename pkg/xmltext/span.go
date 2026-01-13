package xmltext

type span struct {
	buf   *spanBuffer
	Start int
	End   int
	gen   uint32
}

type qnameSpan struct {
	Full      span
	Prefix    span
	Local     span
	HasPrefix bool
}

type attrSpan struct {
	ValueSpan span
	Name      qnameSpan
}

type spanBuffer struct {
	entities *entityResolver
	data     []byte
	gen      uint32
	poison   bool
	stable   bool
}

func makeSpan(buf *spanBuffer, start, end int) span {
	if buf == nil {
		return span{Start: start, End: end}
	}
	return span{Start: start, End: end, buf: buf, gen: buf.gen}
}

func (s span) bytes() []byte {
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

func (s span) bytesUnsafe() []byte {
	if s.buf == nil {
		return nil
	}
	return s.buf.data[s.Start:s.End]
}
