package xmltext

// Attr holds an attribute name and value for a start element token.
// Name and Value are backed by the Token that produced them.
type Attr struct {
	Name  []byte
	Value []byte
	// ValueNeeds reports whether Value includes unresolved entity references.
	ValueNeeds bool
}

// Token is a decoded XML token with caller-owned byte slices.
// Slices are backed by the Token's internal buffers and remain valid until the
// next ReadTokenInto call that reuses the Token.
type Token struct {
	nameBuf      []byte
	Attrs        []Attr
	Text         []byte
	Name         []byte
	attrValueBuf []byte
	attrNameBuf  []byte
	attrsBuf     []Attr
	textBuf      []byte
	Line         int
	Column       int
	TextNeeds    bool
	IsXMLDecl    bool
	Kind         Kind
}

// TokenSizes controls initial buffer capacities.
type TokenSizes struct {
	Name      int
	Text      int
	Attrs     int
	AttrName  int
	AttrValue int
}

func (t *Token) reset() {
	if t == nil {
		return
	}
	t.Name = nil
	t.Attrs = nil
	t.Text = nil
	t.Line = 0
	t.Column = 0
	t.Kind = KindNone
	t.IsXMLDecl = false
	t.TextNeeds = false
	t.resetBuffers()
}

// Reserve ensures the token has at least the requested capacities.
// It resets the buffer lengths to zero.
func (t *Token) Reserve(sizes TokenSizes) {
	if t == nil {
		return
	}
	t.nameBuf = reserveBytes(t.nameBuf, sizes.Name)
	t.textBuf = reserveBytes(t.textBuf, sizes.Text)
	t.attrsBuf = reserveAttrs(t.attrsBuf, sizes.Attrs)
	t.attrNameBuf = reserveBytes(t.attrNameBuf, sizes.AttrName)
	t.attrValueBuf = reserveBytes(t.attrValueBuf, sizes.AttrValue)
	t.reset()
}

func (t *Token) resetBuffers() {
	if t == nil {
		return
	}
	t.nameBuf = t.nameBuf[:0]
	t.textBuf = t.textBuf[:0]
	t.attrsBuf = t.attrsBuf[:0]
	t.attrNameBuf = t.attrNameBuf[:0]
	t.attrValueBuf = t.attrValueBuf[:0]
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
