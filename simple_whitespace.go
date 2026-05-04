package xsd

import "strings"

func normalizeWhitespace(s string, mode whitespaceMode) string {
	if mode == whitespacePreserve {
		return s
	}
	if !containsXMLWhitespace(s) {
		return s
	}
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		isSpace := r == ' ' || r == '\t' || r == '\n' || r == '\r'
		if mode == whitespaceReplace {
			if isSpace {
				b.WriteByte(' ')
			} else {
				b.WriteRune(r)
			}
			continue
		}
		if isSpace {
			lastSpace = true
			continue
		}
		if lastSpace && b.Len() > 0 {
			b.WriteByte(' ')
		}
		lastSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

func normalizeXMLAttributeWhitespace(s string) string {
	if !containsXMLWhitespace(s) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func containsXMLWhitespace(s string) bool {
	for i := 0; i < len(s); i++ {
		if isXMLWhitespaceByte(s[i]) {
			return true
		}
	}
	return false
}

func isXMLWhitespaceByte(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func removeXMLWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		default:
			return r
		}
	}, s)
}
