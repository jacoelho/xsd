package stream

import (
	"bytes"
	"errors"
	"strings"
)

const maxXMLDeclarationPreviewBytes = xmlInputBufferSize

// ErrXMLInputNilReader reports a nil XML input reader.
var ErrXMLInputNilReader = errors.New("xml input reader is nil")

// ErrUnsupportedNonUTF8 reports XML input that declares or uses a non-UTF-8 encoding.
var ErrUnsupportedNonUTF8 = errors.New("xml input must be UTF-8")

// UnsupportedXMLVersionError reports an XML version this tokenizer does not support.
type UnsupportedXMLVersionError struct {
	Version string
}

func (e UnsupportedXMLVersionError) Error() string {
	return "XML version " + e.Version + " is not supported"
}

func (p *Parser) prepareXMLProlog() error {
	peek, err := p.br.ensure(XMLDeclarationPrefixLen)
	if err != nil && !IsOnlyEOF(err) {
		return err
	}
	if HasUTF8BOM(peek) {
		p.br.discardUTF8BOM()
		peek, err = p.br.ensure(XMLDeclarationPrefixLen)
		if err != nil && !IsOnlyEOF(err) {
			return err
		}
	}
	if len(peek) >= 2 {
		if (peek[0] == 0xFE && peek[1] == 0xFF) || (peek[0] == 0xFF && peek[1] == 0xFE) {
			return ErrUnsupportedNonUTF8
		}
	}
	if StartsXMLDeclaration(peek) {
		peek = p.peekXMLDeclaration()
	}
	if enc := DeclaredEncoding(peek); enc != "" && !strings.EqualFold(enc, "UTF-8") && !strings.EqualFold(enc, "UTF8") {
		return ErrUnsupportedNonUTF8
	}
	if version := DeclaredXMLVersion(peek); version != "" && version != xmlVersion10 {
		return UnsupportedXMLVersionError{Version: version}
	}
	return nil
}

func (p *Parser) peekXMLDeclaration() []byte {
	n := XMLDeclarationPrefixLen
	for {
		peek, err := p.br.ensure(n)
		if end := bytes.Index(peek, []byte("?>")); end >= 0 {
			return peek[:end+2]
		}
		if err != nil || n == maxXMLDeclarationPreviewBytes {
			return peek
		}
		n *= 2
		if n > maxXMLDeclarationPreviewBytes {
			n = maxXMLDeclarationPreviewBytes
		}
	}
}
