package xmlstream

import "github.com/jacoelho/xsd/internal/lex"

// referenceContent locates, for the current CharacterDataReference token, the
// first character that is not XML whitespace after decoding. Only references
// update it, so text without references pays nothing; the bytes between two
// references are literal and are walked when the next reference arrives or
// when a diagnostic asks.
type referenceContent struct {
	// firstReference is the Data index of the token's first decoded reference.
	firstReference int
	// Data[:scanned] is whitespace and Data[scanned] is at line:column,
	// unless found, when line:column is the content position itself.
	scanned      int
	line, column int
	found        bool
}

// readCharDataReference decodes one reference into the token buffer for the
// character-data token that starts at line:column and so far has kind; the
// token is a CharacterDataReference token from then on.
func (p *parser) readCharDataReference(kind CharacterDataKind, line, column int) error {
	if kind != CharacterDataReference {
		p.reference = referenceContent{firstReference: len(p.textBuf), line: line, column: column}
	}
	content := &p.reference
	if content.found {
		return p.readEntity(&p.textBuf)
	}
	n, contentLine, contentColumn := skipWhitespace(p.textBuf[content.scanned:], content.line, content.column)
	content.line, content.column = contentLine, contentColumn
	if content.scanned+n < len(p.textBuf) {
		content.found = true
		return p.readEntity(&p.textBuf)
	}
	start := len(p.textBuf)
	if err := p.readEntity(&p.textBuf); err != nil {
		return err
	}
	if !lex.IsXMLWhitespaceBytes(p.textBuf[start:]) {
		// The '&' directly follows the walked whitespace.
		content.found = true
		return nil
	}
	content.scanned = len(p.textBuf)
	content.line, content.column = p.br.nextBytePos()
	return nil
}

// skipWhitespace walks the literal XML whitespace that starts data from source
// position line:column. data holds normalized text, so each line break is one
// '\n'. It returns the length of that whitespace and the position of the byte
// after it.
func skipWhitespace(data []byte, line, column int) (n, nextLine, nextColumn int) {
	for i, b := range data {
		switch b {
		case ' ', '\t':
			column++
		case '\n':
			line++
			column = 1
		default:
			return i, line, column
		}
	}
	return len(data), line, column
}

// contentPos implements Reader.ContentPos for t, the parser's current token.
func (p *parser) contentPos(t *Token) (line, column int) {
	if t.Kind != KindCharData {
		return t.Line, t.Column
	}
	data, line, column := t.Data, t.Line, t.Column
	switch t.TextKind {
	case CharacterDataText:
	case CharacterDataReference:
		if p.reference.found {
			return p.reference.line, p.reference.column
		}
		data, line, column = data[p.reference.scanned:], p.reference.line, p.reference.column
	case CharacterDataCDATA:
		if t.StartOffset == p.cdataOffset {
			column += len("<![CDATA[")
		}
	case CharacterDataInvalid:
		return t.Line, t.Column
	}
	n, line, column := skipWhitespace(data, line, column)
	if n == len(data) {
		return t.Line, t.Column
	}
	return line, column
}

// outsideRootFailure classifies character data t outside the document element
// by its first source byte that is not literal whitespace, since a reference
// is forbidden there even when it decodes to whitespace. It returns nil for
// literal whitespace.
func (p *parser) outsideRootFailure(t *Token) (line, column int, cause error) {
	switch t.TextKind {
	case CharacterDataCDATA:
		return t.Line, t.Column, ErrCDATOutsideRoot
	case CharacterDataInvalid:
		return t.Line, t.Column, ErrTextOutsideRoot
	case CharacterDataText:
		n, line, column := skipWhitespace(t.Data, t.Line, t.Column)
		if n == len(t.Data) {
			return 0, 0, nil
		}
		return line, column, ErrTextOutsideRoot
	case CharacterDataReference:
		literal := t.Data[:p.reference.firstReference]
		n, line, column := skipWhitespace(literal, t.Line, t.Column)
		if n == len(literal) {
			return line, column, ErrReferenceOutsideRoot
		}
		return line, column, ErrTextOutsideRoot
	}
	return t.Line, t.Column, ErrTextOutsideRoot
}
