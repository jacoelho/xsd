package xsd

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
)

const instanceReaderBufferSize = 64 * 1024
const maxXMLDeclarationPreviewBytes = instanceReaderBufferSize
const xmlDeclarationPrefixLen = len("<?xml ")

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func prepareInstanceReaderWithBuffer(r io.Reader, br *bufio.Reader) (*bufio.Reader, error) {
	if r == nil {
		return nil, validation(ErrValidationXML, 0, 0, "", "instance reader is nil")
	}
	if br == nil {
		br = bufio.NewReaderSize(r, instanceReaderBufferSize)
	} else {
		br.Reset(r)
	}
	peek, err := br.Peek(xmlDeclarationPrefixLen)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if bytes.HasPrefix(peek, utf8BOM) {
		if _, discardErr := br.Discard(len(utf8BOM)); discardErr != nil {
			return nil, discardErr
		}
		peek, err = br.Peek(xmlDeclarationPrefixLen)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
	}
	if len(peek) >= 2 {
		if (peek[0] == 0xFE && peek[1] == 0xFF) || (peek[0] == 0xFF && peek[1] == 0xFE) {
			return nil, unsupported(ErrUnsupportedNonUTF8, "instance documents must be UTF-8")
		}
	}
	if startsXMLDeclaration(peek) {
		peek = peekXMLDeclaration(br)
	}
	if enc := declaredEncoding(peek); enc != "" && !strings.EqualFold(enc, "UTF-8") && !strings.EqualFold(enc, "UTF8") {
		return nil, unsupported(ErrUnsupportedNonUTF8, "instance documents must be UTF-8")
	}
	if version := declaredXMLVersion(peek); version != "" && version != xmlVersion10 {
		return nil, unsupported(ErrUnsupportedXML11, "XML version "+version+" is not supported")
	}
	return br, nil
}

func peekXMLDeclaration(br *bufio.Reader) []byte {
	n := xmlDeclarationPrefixLen
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

func startsXMLDeclaration(buf []byte) bool {
	const declLen = len("<?xml")
	return len(buf) > declLen &&
		buf[0] == '<' &&
		buf[1] == '?' &&
		buf[2] == 'x' &&
		buf[3] == 'm' &&
		buf[4] == 'l' &&
		isXMLWhitespaceByte(buf[declLen])
}

func declaredEncoding(buf []byte) string {
	value, ok := declaredXMLDeclAttr(buf, "encoding")
	if !ok {
		return ""
	}
	return value
}

func declaredXMLVersion(buf []byte) string {
	value, ok := declaredXMLDeclAttr(buf, xsdAttrVersion)
	if !ok {
		return ""
	}
	return value
}

func declaredXMLDeclAttr(buf []byte, want string) (string, bool) {
	if !startsXMLDeclaration(buf) {
		return "", false
	}
	content := buf[len("<?xml"):]
	pos := xmlDeclFirstAttr
	for {
		name, value, rest, ok := scanXMLDeclAttr(content, pos)
		if !ok {
			return "", false
		}
		if name == want {
			return value, true
		}
		content = rest
		pos = xmlDeclNextAttr
	}
}
