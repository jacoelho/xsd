package xmlstream

import (
	"bytes"
	"errors"
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

func (p *parser) prepareXMLProlog() error {
	peek, err := p.initialXMLBytes()
	if err != nil {
		return err
	}
	if startsUTF16(peek) {
		return ErrUnsupportedNonUTF8
	}
	if !StartsXMLDeclaration(peek) {
		return nil
	}
	content, complete := bytes.CutSuffix(p.peekXMLDeclaration()[len("<?xml"):], []byte("?>"))
	if !complete {
		return nil
	}
	// Reset reports only version and encoding rejections early; grammar errors
	// are left to the parser so they carry a position. Admission stops at the
	// first defect in document order, so both paths classify a declaration alike.
	err = ValidateXMLDeclContent(content)
	if err == nil {
		return nil
	}
	var version UnsupportedXMLVersionError
	if errors.Is(err, ErrUnsupportedNonUTF8) || errors.As(err, &version) {
		return err
	}
	return nil
}

func (p *parser) initialXMLBytes() ([]byte, error) {
	peek, err := p.br.ensure(XMLDeclarationPrefixLen)
	if err != nil && !IsOnlyEOF(err) {
		return nil, err
	}
	if !HasUTF8BOM(peek) {
		return peek, nil
	}
	p.br.discardUTF8BOM()
	peek, err = p.br.ensure(XMLDeclarationPrefixLen)
	if err != nil && !IsOnlyEOF(err) {
		return nil, err
	}
	return peek, nil
}

func startsUTF16(peek []byte) bool {
	return len(peek) >= 2 &&
		(peek[0] == 0xFE && peek[1] == 0xFF || peek[0] == 0xFF && peek[1] == 0xFE)
}

func (p *parser) peekXMLDeclaration() []byte {
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
