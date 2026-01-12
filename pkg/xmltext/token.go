package xmltext

// Token is an allocation-free view of the next XML token.
// Spans are only valid until the next ReadToken or ReadTokenInto call.
type Token struct {
	Text         Span
	Raw          Span
	Name         QNameSpan
	AttrNeeds    []bool
	AttrRaw      []Span
	AttrRawNeeds []bool
	Attrs        []AttrSpan
	Line         int
	Column       int
	Kind         Kind
	TextNeeds    bool
	TextRawNeeds bool
	IsXMLDecl    bool
}

// Clone returns a copy of the token header without extending span lifetimes.
func (t Token) Clone() Token {
	return t
}
