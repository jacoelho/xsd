package xmltext

// CopySpan appends the span bytes to dst.
func CopySpan(dst []byte, s Span) []byte {
	return append(dst, s.bytes()...)
}

// CopyAttrValue appends an attribute value span to dst.
func CopyAttrValue(dst []byte, a AttrSpan) []byte {
	return CopySpan(dst, a.ValueSpan)
}

// UnescapeInto expands entity references from the span into dst.
func UnescapeInto(dst []byte, s Span) ([]byte, error) {
	data := s.bytes()
	if len(data) == 0 {
		return dst, nil
	}
	resolver := &entityResolver{}
	maxTokenSize := 0
	if s.buf != nil && s.buf.entities != nil {
		resolver = s.buf.entities
		maxTokenSize = resolver.maxTokenSize
	}
	return unescapeInto(dst, data, resolver, maxTokenSize)
}
