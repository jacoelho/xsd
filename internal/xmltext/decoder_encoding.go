package xmltext

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
)

func wrapCharsetReaderFromBufio(reader *bufio.Reader, charsetReader func(label string, r io.Reader) (io.Reader, error)) (io.Reader, error) {
	if err := discardUTF8BOM(reader); err != nil {
		return nil, err
	}
	label, err := detectEncoding(reader)
	if err != nil {
		return nil, err
	}
	if label == "" {
		return reader, nil
	}
	if charsetReader == nil {
		return nil, errUnsupportedEncoding
	}
	decoded, err := charsetReader(label, reader)
	if err != nil {
		return nil, err
	}
	if decoded == nil {
		return nil, errUnsupportedEncoding
	}
	return decoded, nil
}

func discardUTF8BOM(r *bufio.Reader) error {
	peek, err := r.Peek(3)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(peek) >= 3 && peek[0] == 0xEF && peek[1] == 0xBB && peek[2] == 0xBF {
		if _, err := r.Discard(3); err != nil {
			return err
		}
	}
	return nil
}

const maxDeclScan = 1024

func detectEncoding(r *bufio.Reader) (string, error) {
	peek, err := r.Peek(4)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", err
	}
	if len(peek) >= 2 {
		if peek[0] == 0xFE && peek[1] == 0xFF {
			return "utf-16", nil
		}
		if peek[0] == 0xFF && peek[1] == 0xFE {
			return "utf-16", nil
		}
	}
	if len(peek) >= 4 {
		if bytes.Equal(peek[:4], []byte{0x00, 0x3C, 0x00, 0x3F}) {
			return "utf-16be", nil
		}
		if bytes.Equal(peek[:4], []byte{0x3C, 0x00, 0x3F, 0x00}) {
			return "utf-16le", nil
		}
	}
	return detectXMLDeclEncoding(r)
}

func detectXMLDeclEncoding(r *bufio.Reader) (string, error) {
	const prefix = "<?xml"
	peek, err := r.Peek(len(prefix))
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", err
	}
	if len(peek) < len(prefix) || !bytes.Equal(peek, []byte(prefix)) {
		return "", nil
	}
	decl, err := r.Peek(maxDeclScan)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", err
	}
	before, _, ok := bytes.Cut(decl, []byte("?>"))
	if !ok {
		return "", nil
	}
	label := parseXMLDeclEncoding(before)
	if label == "" {
		return "", nil
	}
	if isUTF8Label(label) {
		return "", nil
	}
	return label, nil
}

func parseXMLDeclEncoding(decl []byte) string {
	const prefix = "<?xml"
	if !bytes.HasPrefix(decl, []byte(prefix)) {
		return ""
	}
	data := decl[len(prefix):]
	for {
		data = bytes.TrimLeft(data, " \t\r\n")
		if len(data) == 0 {
			return ""
		}
		name, rest := scanXMLDeclName(data)
		if len(name) == 0 {
			return ""
		}
		data = bytes.TrimLeft(rest, " \t\r\n")
		if len(data) == 0 || data[0] != '=' {
			return ""
		}
		data = bytes.TrimLeft(data[1:], " \t\r\n")
		if len(data) == 0 {
			return ""
		}
		quote := data[0]
		if quote != '\'' && quote != '"' {
			return ""
		}
		data = data[1:]
		end := bytes.IndexByte(data, quote)
		if end < 0 {
			return ""
		}
		value := data[:end]
		data = data[end+1:]
		if bytes.EqualFold(name, []byte("encoding")) {
			return string(value)
		}
	}
}

func scanXMLDeclName(data []byte) ([]byte, []byte) {
	if len(data) == 0 || !isNameStartByte(data[0]) {
		return nil, data
	}
	i := 1
	for i < len(data) && isNameByte(data[i]) {
		i++
	}
	return data[:i], data[i:]
}

func isUTF8Label(label string) bool {
	return strings.EqualFold(label, "utf-8") || strings.EqualFold(label, "utf8")
}
