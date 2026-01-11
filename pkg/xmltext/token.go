package xmltext

// Token is an allocation-free view of the next XML token.
type Token struct {
	kind         Kind
	name         QNameSpan
	attrs        []AttrSpan
	attrNeeds    []bool
	attrRaw      []Span
	attrRawNeeds []bool
	text         Span
	textNeeds    bool
	textRawNeeds bool
	line         int
	column       int
	raw          Span
	isXMLDecl    bool
}

// Kind reports the token kind.
func (t Token) Kind() Kind {
	return t.kind
}

// Name returns the QName span for start and end elements.
func (t Token) Name() QNameSpan {
	return t.name
}

// AttrCount reports the number of attributes on a start element.
func (t Token) AttrCount() int {
	return len(t.attrs)
}

// Attrs returns the attribute spans for a start element.
func (t Token) Attrs() []AttrSpan {
	return t.attrs
}

// AttrsInto appends attribute spans to dst and returns the extended slice.
func (t Token) AttrsInto(dst []AttrSpan) []AttrSpan {
	return append(dst, t.attrs...)
}

// TextSpan returns the span for character data and similar tokens.
func (t Token) TextSpan() Span {
	return t.text
}

// TextNeedsUnescape reports whether TextSpan contains entity references.
func (t Token) TextNeedsUnescape() bool {
	return t.textNeeds
}

// AttrNeedsUnescape reports whether Attrs()[i] contains entity references.
func (t Token) AttrNeedsUnescape(i int) bool {
	if i < 0 || i >= len(t.attrNeeds) {
		return false
	}
	return t.attrNeeds[i]
}

// Line reports the 1-based line where the token starts.
func (t Token) Line() int {
	return t.line
}

// Column reports the 1-based column where the token starts.
func (t Token) Column() int {
	return t.column
}

// Clone returns a copy of the token header without extending span lifetimes.
func (t Token) Clone() Token {
	return t
}
