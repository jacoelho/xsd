package xmltext

// Attr holds an attribute name and value for a start element token.
// Name and Value are owned by the caller-provided TokenBuffer.
type Attr struct {
	Name  []byte
	Value []byte
	// ValueNeeds reports whether Value includes unresolved entity references.
	ValueNeeds bool
}

// Token is a decoded XML token with caller-owned byte slices.
// Slices remain valid until the next ReadTokenInto call that reuses the buffer.
type Token struct {
	Name   []byte
	Attrs  []Attr
	Text   []byte
	Line   int
	Column int
	Kind   Kind
	// IsXMLDecl reports whether this token is the XML declaration.
	IsXMLDecl bool
	// TextNeeds reports whether Text includes unresolved entity references.
	TextNeeds bool
}

// TokenBuffer holds reusable storage for ReadTokenInto results.
// The zero value is ready; use Reserve to preallocate capacity.
type TokenBuffer struct {
	name      []byte
	text      []byte
	attrs     []Attr
	attrName  []byte
	attrValue []byte
}

// TokenBufferSizes controls initial buffer capacities.
type TokenBufferSizes struct {
	Name      int
	Text      int
	Attrs     int
	AttrName  int
	AttrValue int
}

// Reset clears the buffer slices for reuse.
// It retains allocated capacity; assign a zero value to release memory.
func (b *TokenBuffer) Reset() {
	if b == nil {
		return
	}
	b.name = b.name[:0]
	b.text = b.text[:0]
	b.attrs = b.attrs[:0]
	b.attrName = b.attrName[:0]
	b.attrValue = b.attrValue[:0]
}

// Reserve ensures the buffer has at least the requested capacities.
// It resets the buffer lengths to zero.
func (b *TokenBuffer) Reserve(sizes TokenBufferSizes) {
	if b == nil {
		return
	}
	b.name = reserveBytes(b.name, sizes.Name)
	b.text = reserveBytes(b.text, sizes.Text)
	b.attrs = reserveAttrs(b.attrs, sizes.Attrs)
	b.attrName = reserveBytes(b.attrName, sizes.AttrName)
	b.attrValue = reserveBytes(b.attrValue, sizes.AttrValue)
}

func reserveBytes(buf []byte, size int) []byte {
	if size <= 0 {
		return buf[:0]
	}
	if cap(buf) >= size {
		return buf[:0]
	}
	return make([]byte, 0, size)
}

func reserveAttrs(buf []Attr, size int) []Attr {
	if size <= 0 {
		return buf[:0]
	}
	if cap(buf) >= size {
		return buf[:0]
	}
	return make([]Attr, 0, size)
}
