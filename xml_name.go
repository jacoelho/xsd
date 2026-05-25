package xsd

import (
	"bytes"
	"unicode/utf8"
)

func isNameTerminator(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '/', '>', '=':
		return true
	default:
		return false
	}
}

func isXMLNameBytes(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for i, c := range b {
		if c >= utf8.RuneSelf {
			return isXMLNameBytesUnicode(b)
		}
		if i == 0 {
			if !isASCIIXMLNameStart(c) {
				return false
			}
			continue
		}
		if !isASCIIXMLNameChar(c) {
			return false
		}
	}
	return true
}

func isXMLNameBytesUnicode(b []byte) bool {
	first := true
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if first {
			if !isXMLNameStartChar(r) {
				return false
			}
			first = false
		} else if !isXMLNameChar(r) {
			return false
		}
		b = b[size:]
	}
	return true
}

func isNCNameBytes(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for i, c := range b {
		if c >= utf8.RuneSelf {
			return isNCNameBytesUnicode(b)
		}
		if i == 0 {
			if !isASCIINCNameStart(c) {
				return false
			}
			continue
		}
		if !isASCIINCNameChar(c) {
			return false
		}
	}
	return true
}

func isNCNameBytesUnicode(b []byte) bool {
	first := true
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if r == ':' {
			return false
		}
		if first {
			if !isXMLNameStartChar(r) {
				return false
			}
			first = false
		} else if !isXMLNameChar(r) {
			return false
		}
		b = b[size:]
	}
	return true
}

func splitASCIIQNameBytes(b []byte) (prefix, local []byte, ascii, ok bool) {
	if len(b) == 0 {
		return nil, nil, true, false
	}
	colon := -1
	partStart := true
	for i, c := range b {
		if c >= utf8.RuneSelf {
			return nil, nil, false, false
		}
		if c == ':' {
			if colon >= 0 || partStart || i == len(b)-1 {
				return nil, nil, true, false
			}
			colon = i
			partStart = true
			continue
		}
		if partStart {
			if !isASCIINCNameStart(c) {
				return nil, nil, true, false
			}
			partStart = false
			continue
		}
		if !isASCIINCNameChar(c) {
			return nil, nil, true, false
		}
	}
	if colon < 0 {
		return nil, b, true, true
	}
	return b[:colon], b[colon+1:], true, true
}

func isASCIIXMLNameStart(c byte) bool {
	return c == ':' || isASCIINCNameStart(c)
}

func isASCIIXMLNameChar(c byte) bool {
	return isASCIIXMLNameStart(c) || c == '-' || c == '.' || ('0' <= c && c <= '9')
}

func isASCIINCNameStart(c byte) bool {
	return c == '_' || ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z')
}

func isASCIINCNameChar(c byte) bool {
	return isASCIINCNameStart(c) || c == '-' || c == '.' || ('0' <= c && c <= '9')
}

func isDOCTYPEDeclaration(b []byte) bool {
	if len(b) <= len(doctypeDirective) {
		return false
	}
	return bytes.HasPrefix(b, doctypeDirective) && isXMLWhitespaceByte(b[len(doctypeDirective)])
}
