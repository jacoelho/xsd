package xsd

import (
	"iter"
	"slices"
	"strings"
)

func normalizeWhitespace(s string, mode whitespaceMode) string {
	switch mode {
	case whitespacePreserve:
		return s
	case whitespaceReplace:
		return replaceXMLWhitespace(s)
	default:
		return collapseXMLWhitespace(s)
	}
}

func replaceXMLWhitespace(s string) string {
	i := indexNonSpaceXMLWhitespace(s)
	if i < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])
	for ; i < len(s); i++ {
		if isNonSpaceXMLWhitespaceByte(s[i]) {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func collapseXMLWhitespace(s string) string {
	i := firstXMLWhitespaceCollapseChange(s)
	if i < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])
	pendingSpace := false
	for ; i < len(s); i++ {
		if isXMLWhitespaceByte(s[i]) {
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

func trimXMLWhitespace(s string) string {
	start := 0
	for start < len(s) && isXMLWhitespaceByte(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isXMLWhitespaceByte(s[end-1]) {
		end--
	}
	return s[start:end]
}

func trimXMLWhitespaceBytes(b []byte) []byte {
	start := 0
	for start < len(b) && isXMLWhitespaceByte(b[start]) {
		start++
	}
	end := len(b)
	for end > start && isXMLWhitespaceByte(b[end-1]) {
		end--
	}
	return b[start:end]
}

func xmlFieldsSeq(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		start := -1
		for i := 0; i < len(s); i++ {
			if isXMLWhitespaceByte(s[i]) {
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

func indexAnyXMLWhitespace(s string) int {
	for i := 0; i < len(s); i++ {
		if isXMLWhitespaceByte(s[i]) {
			return i
		}
	}
	return -1
}

func indexNonSpaceXMLWhitespace(s string) int {
	for i := 0; i < len(s); i++ {
		if isNonSpaceXMLWhitespaceByte(s[i]) {
			return i
		}
	}
	return -1
}

func firstXMLWhitespaceCollapseChange(s string) int {
	runStart := -1
	runNeedsCollapse := false
	for i := 0; i < len(s); i++ {
		if isXMLWhitespaceByte(s[i]) {
			if runStart < 0 {
				runStart = i
			}
			if i == 0 || isNonSpaceXMLWhitespaceByte(s[i]) {
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

func isXMLWhitespaceByte(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func isNonSpaceXMLWhitespaceByte(b byte) bool {
	switch b {
	case '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func isXMLWhitespaceBytes(data []byte) bool {
	for i := range data {
		if !isXMLWhitespaceByte(data[i]) {
			return false
		}
	}
	return true
}

func hasXMLWhitespaceBytes(data []byte) bool {
	return slices.ContainsFunc(data, isXMLWhitespaceByte)
}

func removeXMLWhitespace(s string) string {
	i := indexAnyXMLWhitespace(s)
	if i < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])
	for ; i < len(s); i++ {
		if !isXMLWhitespaceByte(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
