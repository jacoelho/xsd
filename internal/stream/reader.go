package stream

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
)

const xmlReaderBufferSize = 64 * 1024
const maxXMLDeclarationPreviewBytes = xmlReaderBufferSize

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

// PrepareXMLReaderWithBuffer validates the XML prolog and returns a buffered reader.
func PrepareXMLReaderWithBuffer(r io.Reader, br *bufio.Reader) (*bufio.Reader, error) {
	if r == nil {
		return nil, ErrXMLInputNilReader
	}
	if br == nil {
		br = bufio.NewReaderSize(r, xmlReaderBufferSize)
	} else {
		br.Reset(r)
	}
	peek, err := br.Peek(XMLDeclarationPrefixLen)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if bytes.HasPrefix(peek, UTF8BOM) {
		if _, discardErr := br.Discard(len(UTF8BOM)); discardErr != nil {
			return nil, discardErr
		}
		peek, err = br.Peek(XMLDeclarationPrefixLen)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
	}
	if len(peek) >= 2 {
		if (peek[0] == 0xFE && peek[1] == 0xFF) || (peek[0] == 0xFF && peek[1] == 0xFE) {
			return nil, ErrUnsupportedNonUTF8
		}
	}
	if StartsXMLDeclaration(peek) {
		peek = peekXMLDeclaration(br)
	}
	if enc := DeclaredEncoding(peek); enc != "" && !strings.EqualFold(enc, "UTF-8") && !strings.EqualFold(enc, "UTF8") {
		return nil, ErrUnsupportedNonUTF8
	}
	if version := DeclaredXMLVersion(peek); version != "" && version != xmlVersion10 {
		return nil, UnsupportedXMLVersionError{Version: version}
	}
	return br, nil
}

func peekXMLDeclaration(br *bufio.Reader) []byte {
	n := XMLDeclarationPrefixLen
	for {
		peek, err := br.Peek(n)
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
