package compile

import "strings"

// TranslateXSDRegexToGo translates XSD regex syntax to the Go regexp subset
// used by compiled pattern facets. Callers must validate syntax first.
func TranslateXSDRegexToGo(source string) string {
	translator := xsdRegexTranslator{source: source}
	for i := 0; i < len(source); i++ {
		i = translator.consume(i)
	}
	if translator.escaped {
		translator.output.WriteByte('\\')
	}
	return translator.output.String()
}

type xsdRegexTranslator struct {
	source  string
	output  strings.Builder
	escaped bool
	inClass bool
}

func (t *xsdRegexTranslator) consume(index int) int {
	c := t.source[index]
	if t.escaped {
		t.writeEscaped(c)
		return index
	}
	if c == '\\' {
		t.escaped = true
		return index
	}
	if t.inClass {
		t.inClass = c != ']'
		t.output.WriteByte(c)
		return index
	}
	if c == '[' {
		t.inClass = true
		t.output.WriteByte(c)
		return index
	}
	return t.consumeOutsideClass(index, c)
}

func (t *xsdRegexTranslator) writeEscaped(c byte) {
	if !t.writeClassEscape(c) {
		t.output.WriteByte('\\')
		t.output.WriteByte(c)
	}
	t.escaped = false
}

func (t *xsdRegexTranslator) consumeOutsideClass(index int, c byte) int {
	if c == '{' {
		if end, ok := t.writeQuantifier(index); ok {
			return end
		}
	}
	if c == '.' {
		// XSD '.' matches any character except newline and carriage
		// return; Go '.' only excludes newline.
		t.output.WriteString(`[^\n\r]`)
		return index
	}
	if c == '^' || c == '$' {
		t.output.WriteByte('\\')
	}
	t.output.WriteByte(c)
	return index
}

func (t *xsdRegexTranslator) writeQuantifier(index int) (int, bool) {
	end := strings.IndexByte(t.source[index:], '}')
	if end < 0 {
		return index, false
	}
	end += index
	t.output.WriteString(normalizeXSDRegexQuantifier(t.source[index : end+1]))
	return end, true
}

func (t *xsdRegexTranslator) writeClassEscape(c byte) bool {
	switch c {
	case 'd':
		t.writeClass(xsdDigitClassInner)
	case 'D':
		t.writeNegatedClass(xsdDigitClassInner)
	case 's':
		t.writeClass(xsdSpaceClassInner)
	case 'S':
		t.writeNegatedClass(xsdSpaceClassInner)
	case 'w':
		t.writeClass(xsdWordClassInner)
	case 'W':
		t.writeClass(xsdNotWordClassInner)
	default:
		return false
	}
	return true
}

func (t *xsdRegexTranslator) writeClass(inner string) {
	if t.inClass {
		t.output.WriteString(inner)
		return
	}
	t.output.WriteByte('[')
	t.output.WriteString(inner)
	t.output.WriteByte(']')
}

func (t *xsdRegexTranslator) writeNegatedClass(inner string) {
	if t.inClass {
		t.output.WriteByte('^')
		t.output.WriteString(inner)
		return
	}
	t.output.WriteString(`[^`)
	t.output.WriteString(inner)
	t.output.WriteByte(']')
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

// The \w and \W classes exclude U+023F explicitly: the W3C XSD test suite
// (w3c/msMeta/Regex_w3c.xml, case reU6) is authored against pre-4.1 Unicode
// tables where U+023F was unassigned (category Cn, hence \p{C}), while Go's
// current tables classify it as a letter.
const xsdWordClassInner = `^\pP\pZ\pC\x{023F}`

const xsdNotWordClassInner = `\pP\pZ\pC\x{023F}`
