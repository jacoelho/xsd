// Package lex owns shared XML lexical predicates.
package lex

import (
	"iter"
	"slices"
	"strings"
	"unicode/utf8"
)

// IsNameTerminator reports whether b ends an XML name in a tag context.
func IsNameTerminator(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '/', '>', '=':
		return true
	default:
		return false
	}
}

// IsXMLWhitespaceByte reports whether b is XML whitespace.
func IsXMLWhitespaceByte(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

// IsNonSpaceXMLWhitespaceByte reports whether b is XML whitespace other than space.
func IsNonSpaceXMLWhitespaceByte(b byte) bool {
	switch b {
	case '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

// TrimXMLWhitespaceBytes trims XML whitespace from both ends of b.
func TrimXMLWhitespaceBytes(b []byte) []byte {
	start := 0
	for start < len(b) && IsXMLWhitespaceByte(b[start]) {
		start++
	}
	end := len(b)
	for end > start && IsXMLWhitespaceByte(b[end-1]) {
		end--
	}
	return b[start:end]
}

// TrimXMLWhitespaceString trims XML whitespace from both ends of s.
func TrimXMLWhitespaceString(s string) string {
	start := 0
	for start < len(s) && IsXMLWhitespaceByte(s[start]) {
		start++
	}
	end := len(s)
	for end > start && IsXMLWhitespaceByte(s[end-1]) {
		end--
	}
	return s[start:end]
}

// IsXMLWhitespaceBytes reports whether all bytes in data are XML whitespace.
func IsXMLWhitespaceBytes(data []byte) bool {
	for i := range data {
		if !IsXMLWhitespaceByte(data[i]) {
			return false
		}
	}
	return true
}

// HasXMLWhitespaceBytes reports whether data contains any XML whitespace.
func HasXMLWhitespaceBytes(data []byte) bool {
	return slices.ContainsFunc(data, IsXMLWhitespaceByte)
}

// ReplaceXMLWhitespace replaces non-space XML whitespace with spaces.
func ReplaceXMLWhitespace(s string) string {
	i := indexNonSpaceXMLWhitespace(s)
	if i < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])
	for ; i < len(s); i++ {
		if IsNonSpaceXMLWhitespaceByte(s[i]) {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// CollapseXMLWhitespace replaces XML whitespace runs with one space and trims both ends.
func CollapseXMLWhitespace(s string) string {
	i := firstXMLWhitespaceCollapseChange(s)
	if i < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])
	pendingSpace := false
	for ; i < len(s); i++ {
		if IsXMLWhitespaceByte(s[i]) {
			if b.Len() > 0 {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace {
			b.WriteByte(' ')
			pendingSpace = false
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// XMLFieldsSeq returns fields split on XML whitespace.
func XMLFieldsSeq(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		start := -1
		for i := range len(s) {
			if IsXMLWhitespaceByte(s[i]) {
				if start >= 0 {
					if !yield(s[start:i]) {
						return
					}
					start = -1
				}
				continue
			}
			if start < 0 {
				start = i
			}
		}
		if start >= 0 {
			yield(s[start:])
		}
	}
}

func firstXMLWhitespaceCollapseChange(s string) int {
	runStart := -1
	runNeedsCollapse := false
	for i := range len(s) {
		if IsXMLWhitespaceByte(s[i]) {
			if runStart < 0 {
				runStart = i
			}
			if i == 0 || IsNonSpaceXMLWhitespaceByte(s[i]) {
				runNeedsCollapse = true
			}
			continue
		}

		if runStart < 0 {
			continue
		}
		if runNeedsCollapse || i-runStart > 1 {
			return runStart
		}
		runStart = -1
		runNeedsCollapse = false
	}
	if runStart >= 0 {
		return runStart
	}
	return -1
}

func indexNonSpaceXMLWhitespace(s string) int {
	for i := range len(s) {
		if IsNonSpaceXMLWhitespaceByte(s[i]) {
			return i
		}
	}
	return -1
}

// IsXMLChar reports whether r is an XML character.
func IsXMLChar(r rune) bool {
	return r == '\t' ||
		r == '\n' ||
		r == '\r' ||
		(r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0x10FFFF)
}

// IsXMLNameStartChar reports whether r can start an XML Name.
func IsXMLNameStartChar(r rune) bool {
	return r == ':' ||
		r == '_' ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 0xC0 && r <= 0xD6) ||
		(r >= 0xD8 && r <= 0xF6) ||
		(r >= 0xF8 && r <= 0x2FF) ||
		(r >= 0x370 && r <= 0x37D) ||
		(r >= 0x37F && r <= 0x1FFF) ||
		(r >= 0x200C && r <= 0x200D) ||
		(r >= 0x2070 && r <= 0x218F) ||
		(r >= 0x2C00 && r <= 0x2FEF) ||
		(r >= 0x3001 && r <= 0xD7FF) ||
		(r >= 0xF900 && r <= 0xFDCF) ||
		(r >= 0xFDF0 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0xEFFFF)
}

// IsXMLNameChar reports whether r can appear in an XML Name.
func IsXMLNameChar(r rune) bool {
	return IsXMLNameStartChar(r) ||
		r == '-' ||
		r == '.' ||
		(r >= '0' && r <= '9') ||
		r == 0xB7 ||
		(r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x203F && r <= 0x2040)
}

// IsXMLName reports whether s is an XML Name.
func IsXMLName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !IsXMLNameStartChar(r) {
				return false
			}
			continue
		}
		if !IsXMLNameChar(r) {
			return false
		}
	}
	return true
}

// IsNCName reports whether s is an XML NCName.
func IsNCName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == ':' {
			return false
		}
		if i == 0 {
			if !IsXMLNameStartChar(r) {
				return false
			}
			continue
		}
		if !IsXMLNameChar(r) {
			return false
		}
	}
	return true
}

// IsNMTOKEN reports whether s is an XML NMTOKEN.
func IsNMTOKEN(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !IsXMLNameChar(r) {
			return false
		}
	}
	return true
}

// IsLanguage reports whether s matches the lexical space of xs:language.
func IsLanguage(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	for part := range strings.SplitSeq(s, "-") {
		if part == "" || len(part) > 8 {
			return false
		}
		for j := range len(part) {
			c := part[j]
			if i == 0 {
				if !isASCIILetter(c) {
					return false
				}
				continue
			}
			if !isASCIILetter(c) && !isASCIIDigit(c) {
				return false
			}
		}
		i++
	}
	return true
}

// IsXMLNameBytes reports whether b is an XML Name.
func IsXMLNameBytes(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for i, c := range b {
		if c >= utf8.RuneSelf {
			return isXMLNameBytesUnicode(b)
		}
		if i == 0 {
			if !IsASCIIXMLNameStart(c) {
				return false
			}
			continue
		}
		if !IsASCIIXMLNameChar(c) {
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
			if !IsXMLNameStartChar(r) {
				return false
			}
			first = false
		} else if !IsXMLNameChar(r) {
			return false
		}
		b = b[size:]
	}
	return true
}

// IsNCNameBytes reports whether b is an XML NCName.
func IsNCNameBytes(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for i, c := range b {
		if c >= utf8.RuneSelf {
			return isNCNameBytesUnicode(b)
		}
		if i == 0 {
			if !IsASCIINCNameStart(c) {
				return false
			}
			continue
		}
		if !IsASCIINCNameChar(c) {
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
			if !IsXMLNameStartChar(r) {
				return false
			}
			first = false
		} else if !IsXMLNameChar(r) {
			return false
		}
		b = b[size:]
	}
	return true
}

// IsNMTOKENBytes reports whether b is an XML NMTOKEN.
func IsNMTOKENBytes(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for len(b) > 0 {
		c := b[0]
		if c < utf8.RuneSelf {
			if !IsASCIIXMLNameChar(c) {
				return false
			}
			b = b[1:]
			continue
		}
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if !IsXMLNameChar(r) {
			return false
		}
		b = b[size:]
	}
	return true
}

// SplitASCIIQNameBytes splits an ASCII QName into prefix and local parts.
func SplitASCIIQNameBytes(b []byte) (prefix, local []byte, ascii, ok bool) {
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
			if !IsASCIINCNameStart(c) {
				return nil, nil, true, false
			}
			partStart = false
			continue
		}
		if !IsASCIINCNameChar(c) {
			return nil, nil, true, false
		}
	}
	if colon < 0 {
		return nil, b, true, true
	}
	return b[:colon], b[colon+1:], true, true
}

// IsASCIIXMLNameStart reports whether c starts an ASCII XML Name.
func IsASCIIXMLNameStart(c byte) bool {
	return c == ':' || IsASCIINCNameStart(c)
}

// IsASCIIXMLNameChar reports whether c can appear in an ASCII XML Name.
func IsASCIIXMLNameChar(c byte) bool {
	return IsASCIIXMLNameStart(c) || c == '-' || c == '.' || ('0' <= c && c <= '9')
}

// IsASCIINCNameStart reports whether c starts an ASCII XML NCName.
func IsASCIINCNameStart(c byte) bool {
	return c == '_' || ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z')
}

// IsASCIINCNameChar reports whether c can appear in an ASCII XML NCName.
func IsASCIINCNameChar(c byte) bool {
	return IsASCIINCNameStart(c) || c == '-' || c == '.' || ('0' <= c && c <= '9')
}

func isASCIILetter(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
