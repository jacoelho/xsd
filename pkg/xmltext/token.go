package xmltext

// Attr holds an attribute name and value for a start element token.
// Name and Value are backed by the Token that produced them.
type Attr struct {
	Name      []byte
	Value     []byte
	NameColon int
	// ValueNeeds reports whether Value includes unresolved entity references.
	ValueNeeds bool
}

// RawAttr holds attribute bytes backed by the decoder buffer.
type RawAttr struct {
	Name       []byte
	Value      []byte
	NameColon  int
	ValueNeeds bool
}

// Token is a decoded XML token with caller-owned byte slices.
// Slices are backed by the Token's internal buffers and remain valid until the
// next ReadTokenInto call that reuses the Token.
type Token struct {
	attrNameBuf  []byte
	Attrs        []Attr
	Text         []byte
	Name         []byte
	attrsBuf     []Attr
	attrValueBuf []byte
	textBuf      []byte
	nameBuf      []byte
	Column       int
	Line         int
	NameColon    int
	IsXMLDecl    bool
	TextNeeds    bool
	Kind         Kind
}

// RawToken is a decoded XML token backed by the decoder buffer.
// Slices are valid until the next ReadTokenRawInto call that reuses the token.
type RawToken struct {
	Attrs     []RawAttr
	Text      []byte
	Name      []byte
	attrsBuf  []RawAttr
	NameColon int
	Line      int
	Column    int
	TextNeeds bool
	IsXMLDecl bool
	Kind      Kind
}

// RawTokenSpan is a decoded XML token backed by the decoder buffer with
// attribute spans accessible by index without conversion.
// Slices are valid until the next ReadTokenRawSpansInto call that reuses the token.
type RawTokenSpan struct {
	Name      []byte
	Text      []byte
	attrs     []attrSpan
	attrNeeds []bool
	NameColon int
	Line      int
	Column    int
	TextNeeds bool
	IsXMLDecl bool
	Kind      Kind
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
	t.NameColon = -1
	t.Line = 0
	t.Column = 0
	t.Kind = KindNone
	t.IsXMLDecl = false
	t.TextNeeds = false
	t.resetBuffers()
}

func (t *RawToken) reset() {
	if t == nil {
		return
	}
	t.Name = nil
	t.Attrs = nil
	t.Text = nil
	t.NameColon = -1
	t.Line = 0
	t.Column = 0
	t.Kind = KindNone
	t.IsXMLDecl = false
	t.TextNeeds = false
	t.resetBuffers()
}

// AttrCount returns the number of attributes on the token.
func (t *RawTokenSpan) AttrCount() int {
	if t == nil {
		return 0
	}
	return len(t.attrs)
}

// AttrName returns the raw attribute name bytes at index i.
func (t *RawTokenSpan) AttrName(i int) []byte {
	return t.attrs[i].Name.Full.bytesUnsafe()
}

// AttrNameColon returns the colon index for the attribute name at index i, or -1.
func (t *RawTokenSpan) AttrNameColon(i int) int {
	name := t.attrs[i].Name
	if name.HasPrefix {
		return name.Prefix.End - name.Prefix.Start
	}
	return -1
}

// AttrValue returns the raw attribute value bytes at index i.
func (t *RawTokenSpan) AttrValue(i int) []byte {
	return t.attrs[i].ValueSpan.bytesUnsafe()
}

// AttrValueNeeds reports whether the attribute value at index i includes unresolved entities.
func (t *RawTokenSpan) AttrValueNeeds(i int) bool {
	return t.attrNeeds[i]
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

func (t *RawToken) resetBuffers() {
	if t == nil {
		return
	}
	t.attrsBuf = t.attrsBuf[:0]
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

// Reserve preallocates token storage for upcoming parse operations.
func (t *RawToken) Reserve(sizes TokenSizes) {
	if t == nil {
		return
	}
	t.attrsBuf = reserveRawAttrs(t.attrsBuf, sizes.Attrs)
	t.reset()
}

func reserveRawAttrs(buf []RawAttr, size int) []RawAttr {
	if size <= 0 {
		return buf[:0]
	}
	if cap(buf) >= size {
		return buf[:0]
	}
	return make([]RawAttr, 0, size)
}
