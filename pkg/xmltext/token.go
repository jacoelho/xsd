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
type TokenBuffer struct {
	Name      []byte
	Text      []byte
	Attrs     []Attr
	AttrName  []byte
	AttrValue []byte
}

// Reset clears the buffer slices for reuse.
func (b *TokenBuffer) Reset() {
	if b == nil {
		return
	}
	b.Name = b.Name[:0]
	b.Text = b.Text[:0]
	b.Attrs = b.Attrs[:0]
	b.AttrName = b.AttrName[:0]
	b.AttrValue = b.AttrValue[:0]
}
