package xmlstream

type namespaceBytesCache struct {
	index map[string]nsSpan
	blob  []byte
}

type nsSpan struct {
	off uint32
	len uint32
}

func (c *namespaceBytesCache) reset() {
	if c == nil {
		return
	}
	c.blob = c.blob[:0]
	if c.index == nil {
		c.index = make(map[string]nsSpan, 32)
		return
	}
	clear(c.index)
}

func (c *namespaceBytesCache) intern(namespace string) []byte {
	if namespace == "" {
		return nil
	}
	if c.index == nil {
		c.index = make(map[string]nsSpan, 32)
	}
	if span, ok := c.index[namespace]; ok {
		off := span.off
		end := off + span.len
		if int(end) <= len(c.blob) {
			return c.blob[off:end]
		}
	}
	off := uint32(len(c.blob))
	c.blob = append(c.blob, namespace...)
	span := nsSpan{off: off, len: uint32(len(namespace))}
	c.index[namespace] = span
	return c.blob[span.off : span.off+span.len]
}
