package xsd

import (
	"strings"
	"unicode"
)

func isXMLName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !isXMLNameStartChar(r) {
				return false
			}
			continue
		}
		if !isXMLNameChar(r) {
			return false
		}
	}
	return true
}

func isXMLChar(r rune) bool {
	return r == '\t' ||
		r == '\n' ||
		r == '\r' ||
		(r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0x10FFFF)
}

func isXMLNameStartChar(r rune) bool {
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

func isXMLNameChar(r rune) bool {
	return isXMLNameStartChar(r) ||
		r == '-' ||
		r == '.' ||
		(r >= '0' && r <= '9') ||
		r == 0xB7 ||
		(r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x203F && r <= 0x2040)
}

func isNCName(s string) bool {
	return isXMLName(s) && !strings.Contains(s, ":")
}

func isNMTOKEN(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isXMLNameChar(r) {
			return false
		}
	}
	return true
}

func isLanguage(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	for part := range strings.SplitSeq(s, "-") {
		if part == "" || len(part) > 8 {
			return false
		}
		for _, r := range part {
			if i == 0 {
				if !unicode.IsLetter(r) {
					return false
				}
				continue
			}
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return false
			}
		}
		i++
	}
	return true
}
