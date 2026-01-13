package xmltext

type rawToken struct {
	// text is the token payload span (text/CDATA/PI/comment/directive).
	text span
	// raw is the raw token span in the input buffer.
	raw span
	// name is the element QName span for start/end tokens.
	name qnameSpan
	// attrNeeds reports unresolved entity references per attribute.
	attrNeeds []bool
	// attrRaw holds raw attribute value spans in the token.
	attrRaw []span
	// attrRawNeeds reports entity references per raw attribute value.
	attrRawNeeds []bool
	// attrs holds attribute spans for start elements.
	attrs []attrSpan
	// line is the 1-based line where the token starts when tracking is enabled.
	line int
	// column is the 1-based column where the token starts when tracking is enabled.
	column int
	// kind is the token kind.
	kind Kind
	// textNeeds reports unresolved entity references in text.
	textNeeds bool
	// textRawNeeds reports entity references in the raw text span.
	textRawNeeds bool
	// isXMLDecl reports whether a PI token was an XML declaration.
	isXMLDecl bool
}
