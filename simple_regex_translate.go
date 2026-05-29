package xsd

import "strings"

func translateXSDRegexToGo(source string) string {
	var b strings.Builder
	escaped := false
	inClass := false
	for i := 0; i < len(source); i++ {
		c := source[i]
		if escaped {
			if !writeXSDRegexClassEscape(&b, c, inClass) {
				b.WriteByte('\\')
				b.WriteByte(c)
			}
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		switch {
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case !inClass && c == '{':
			end := strings.IndexByte(source[i:], '}')
			if end >= 0 {
				end += i
				b.WriteString(normalizeXSDRegexQuantifier(source[i : end+1]))
				i = end
				continue
			}
		case !inClass && (c == '^' || c == '$'):
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

func writeXSDRegexClassEscape(b *strings.Builder, c byte, inClass bool) bool {
	switch c {
	case 'd':
		writeXSDRegexClass(b, xsdDigitClassInner, inClass)
	case 'D':
		writeNegatedXSDRegexClass(b, xsdDigitClassInner, inClass)
	case 's':
		writeXSDRegexClass(b, xsdSpaceClassInner, inClass)
	case 'S':
		writeNegatedXSDRegexClass(b, xsdSpaceClassInner, inClass)
	case 'w':
		writeXSDRegexClass(b, xsdWordClassInner, inClass)
	case 'W':
		writeXSDRegexClass(b, xsdNotWordClassInner, inClass)
	default:
		return false
	}
	return true
}

func writeXSDRegexClass(b *strings.Builder, inner string, inClass bool) {
	if inClass {
		b.WriteString(inner)
		return
	}
	b.WriteByte('[')
	b.WriteString(inner)
	b.WriteByte(']')
}

func writeNegatedXSDRegexClass(b *strings.Builder, inner string, inClass bool) {
	if inClass {
		b.WriteByte('^')
		b.WriteString(inner)
		return
	}
	b.WriteString(`[^`)
	b.WriteString(inner)
	b.WriteByte(']')
}

func normalizeXSDRegexQuantifier(s string) string {
	if len(s) < 3 || s[0] != '{' || s[len(s)-1] != '}' {
		return s
	}
	body := s[1 : len(s)-1]
	lower, upper, found := strings.Cut(body, ",")
	if !found {
		return "{" + trimRegexQuantityText(lower) + "}"
	}
	if upper == "" {
		return "{" + trimRegexQuantityText(lower) + ",}"
	}
	return "{" + trimRegexQuantityText(lower) + "," + trimRegexQuantityText(upper) + "}"
}

func trimRegexQuantityText(s string) string {
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	return s
}

const xsdDigitClassInner = `\x{0030}-\x{0039}\x{0660}-\x{0669}\x{06F0}-\x{06F9}\x{0966}-\x{096F}\x{09E6}-\x{09EF}\x{0A66}-\x{0A6F}\x{0AE6}-\x{0AEF}\x{0B66}-\x{0B6F}\x{0BE7}-\x{0BEF}\x{0C66}-\x{0C6F}\x{0CE6}-\x{0CEF}\x{0D66}-\x{0D6F}\x{0E50}-\x{0E59}\x{0ED0}-\x{0ED9}\x{0F20}-\x{0F29}\x{1040}-\x{1049}\x{1369}-\x{1371}\x{17E0}-\x{17E9}\x{1810}-\x{1819}\x{1D7CE}-\x{1D7FF}\x{FF10}-\x{FF19}`

const xsdSpaceClassInner = `\x{0009}\x{000A}\x{000D}\x{0020}`

const xsdWordClassInner = `^\pP\pZ\pC\x{023F}`

const xsdNotWordClassInner = `\pP\pZ\pC\x{023F}`
