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
	return bytes.IndexByte(b, ':') < 0 && isXMLNameBytes(b)
}

func isDOCTYPEDeclaration(b []byte) bool {
	if len(b) <= len(doctypeDirective) {
		return false
	}
	return bytes.HasPrefix(b, doctypeDirective) && isXMLWhitespaceByte(b[len(doctypeDirective)])
}
