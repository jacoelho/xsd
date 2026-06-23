// Package stream tokenizes XML while making borrowed token lifetimes explicit.
package stream

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
)

const (
	xmlPrefix      = vocab.XMLPrefix
	xsdAttrVersion = vocab.XSDAttrVersion
	xmlVersion10   = vocab.XMLVersion10
)

func resetRetainedSlice[T any](s []T) []T {
	const maxRetainedSliceCap = 4096
	if cap(s) > maxRetainedSliceCap {
		return nil
	}
	clear(s[:cap(s)])
	return s[:0]
}

func resetRetainedBytes(s []byte) []byte {
	const maxRetainedBufferCap = 1 << 20
	if cap(s) > maxRetainedBufferCap {
		return nil
	}
	clear(s)
	return s[:0]
}

func stringBytesEqual(s string, b []byte) bool {
	if len(s) != len(b) {
		return false
	}
	for i := range b {
		if s[i] != b[i] {
			return false
		}
	}
	return true
}

// IsDOCTYPEDeclaration reports whether b is a DOCTYPE declaration body.
func IsDOCTYPEDeclaration(b []byte) bool {
	if len(b) <= len(doctypeDirective) {
		return false
	}
	return bytes.HasPrefix(b, doctypeDirective) && lex.IsXMLWhitespaceByte(b[len(doctypeDirective)])
}
