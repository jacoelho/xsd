package xsd

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strings"
)

const instanceReaderBufferSize = 64 * 1024

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func prepareInstanceReader(r io.Reader) (io.Reader, error) {
	br := bufio.NewReaderSize(r, instanceReaderBufferSize)
	peek, _ := br.Peek(512)
	if bytes.HasPrefix(peek, utf8BOM) {
		if _, err := br.Discard(len(utf8BOM)); err != nil {
			return nil, err
		}
		peek = peek[len(utf8BOM):]
	}
	if len(peek) >= 2 {
		if (peek[0] == 0xFE && peek[1] == 0xFF) || (peek[0] == 0xFF && peek[1] == 0xFE) {
			return nil, unsupported(ErrUnsupportedNonUTF8, "instance documents must be UTF-8")
		}
	}
	if enc := declaredEncoding(peek); enc != "" && !strings.EqualFold(enc, "UTF-8") && !strings.EqualFold(enc, "UTF8") {
		return nil, unsupported(ErrUnsupportedNonUTF8, "instance documents must be UTF-8")
	}
	if version := declaredXMLVersion(peek); version != "" && version != "1.0" {
		return nil, unsupported(ErrUnsupportedXML11, "XML version "+version+" is not supported")
	}
	return br, nil
}

var xmlEncodingRE = regexp.MustCompile(`^<\?xml\s+[^>]*encoding\s*=\s*['"]([^'"]+)['"]`)

var xmlVersionRE = regexp.MustCompile(`^<\?xml\s+[^>]*version\s*=\s*['"]([^'"]+)['"]`)

func declaredEncoding(buf []byte) string {
	m := xmlEncodingRE.FindSubmatch(buf)
	if len(m) == 2 {
		return string(m[1])
	}
	return ""
}

func declaredXMLVersion(buf []byte) string {
	m := xmlVersionRE.FindSubmatch(buf)
	if len(m) == 2 {
		return string(m[1])
	}
	return ""
}
