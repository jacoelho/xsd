package model

import "fmt"

func (t *patternTranslator) handleEscape() (bool, error) {
	if t.pattern[t.i] != '\\' {
		return false, nil
	}
	if t.i+1 >= len(t.pattern) {
		return true, fmt.Errorf("pattern-syntax-error: escape sequence at end of pattern")
	}
	nextChar := t.pattern[t.i+1]

	if nextChar == 'u' {
		return true, fmt.Errorf("pattern-syntax-error: \\u escape is not valid XSD 1.0 syntax (use XML character reference &#x; instead)")
	}

	if nextChar == 'p' || nextChar == 'P' {
		translated, newIdx, err := translateUnicodePropertyEscape(t.pattern, t.i, t.inCharClass())
		if err != nil {
			return true, err
		}
		if t.inCharClass() {
			t.classBuf.WriteString(translated)
			t.classState.markNonChar()
		} else {
			t.result.WriteString(translated)
		}
		t.i = newIdx
		t.justWroteQuantifier = false
		return true, nil
	}

	if t.handleNameEscape(nextChar) {
		return true, nil
	}
	if t.handleDigitEscape(nextChar) {
		return true, nil
	}
	if handled, err := t.handleWhitespaceEscape(nextChar); handled {
		return true, err
	}
	if handled, err := t.handleWordEscape(nextChar); handled {
		return true, err
	}
	if handled, err := t.handleControlEscape(nextChar); handled {
		return true, err
	}
	if handled, err := t.handleUnsupportedAnchorEscape(nextChar); handled {
		return true, err
	}
	if handled, err := t.handleEscapedBackslash(nextChar); handled {
		return true, err
	}
	if handled, err := t.handleEscapedMetachar(nextChar); handled {
		return true, err
	}
	if handled, err := t.handleEscapedDash(nextChar); handled {
		return true, err
	}

	if nextChar >= '0' && nextChar <= '9' {
		return true, fmt.Errorf("pattern-syntax-error: \\%c backreference is not valid XSD 1.0 syntax", nextChar)
	}
	return true, fmt.Errorf("pattern-syntax-error: \\%c is not a valid XSD 1.0 escape sequence", nextChar)
}

func (t *patternTranslator) handleNameEscape(nextChar byte) bool {
	switch nextChar {
	case 'i':
		if t.inCharClass() {
			t.classBuf.WriteString(nameStartCharClassContent)
			t.classState.markNonChar()
		} else {
			t.result.WriteString(nameStartCharClass)
		}
	case 'I':
		if t.inCharClass() {
			t.classHasNotNameStart = true
			t.classState.markNonChar()
		} else {
			t.result.WriteString(nameNotStartCharClass)
		}
	case 'c':
		if t.inCharClass() {
			t.classBuf.WriteString(nameCharClassContent)
			t.classState.markNonChar()
		} else {
			t.result.WriteString(nameCharClass)
		}
	case 'C':
		if t.inCharClass() {
			t.classHasNotNameChar = true
			t.classState.markNonChar()
		} else {
			t.result.WriteString(nameNotCharClass)
		}
	default:
		return false
	}

	t.i += 2
	t.justWroteQuantifier = false
	return true
}

func (t *patternTranslator) handleDigitEscape(nextChar byte) bool {
	switch nextChar {
	case 'd':
		if t.inCharClass() {
			t.classBuf.WriteString(xsdDigitClassContent)
			t.classState.markNonChar()
		} else {
			t.result.WriteString(xsdDigitClass)
		}
	case 'D':
		if t.inCharClass() {
			t.classHasNotD = true
			t.classState.markNonChar()
		} else {
			t.result.WriteString(xsdNotDigitClass)
		}
	default:
		return false
	}

	t.i += 2
	t.justWroteQuantifier = false
	return true
}

func (t *patternTranslator) handleWhitespaceEscape(nextChar byte) (bool, error) {
	switch nextChar {
	case 's':
		if t.inCharClass() {
			t.classBuf.WriteString(`\x20\t\n\r`)
			t.classState.markNonChar()
		} else {
			t.result.WriteString(`[\x20\t\n\r]`)
		}
	case 'S':
		if t.inCharClass() {
			if t.classNegated {
				return true, fmt.Errorf("pattern-unsupported: \\S inside negated character class not expressible in RE2")
			}
			t.classHasS = true
			t.classState.markNonChar()
		} else {
			t.result.WriteString(`[^\x20\t\n\r]`)
		}
	default:
		return false, nil
	}

	t.i += 2
	t.justWroteQuantifier = false
	return true, nil
}

func (t *patternTranslator) handleWordEscape(nextChar byte) (bool, error) {
	switch nextChar {
	case 'w':
		if t.inCharClass() {
			if t.classNegated {
				return true, fmt.Errorf("pattern-unsupported: \\w inside negated character class not expressible in RE2")
			}
			t.classHasW = true
			t.classState.markNonChar()
		} else {
			t.result.WriteString(xsdWordClass)
		}
	case 'W':
		if t.inCharClass() {
			t.classBuf.WriteString(`\p{P}\p{Z}\p{C}`)
			t.classState.markNonChar()
		} else {
			t.result.WriteString(xsdNotWordClass)
		}
	default:
		return false, nil
	}

	t.i += 2
	t.justWroteQuantifier = false
	return true, nil
}

func (t *patternTranslator) handleControlEscape(nextChar byte) (bool, error) {
	var char rune
	switch nextChar {
	case 'n':
		char = '\n'
	case 'r':
		char = '\r'
	case 't':
		char = '\t'
	case 'f':
		char = '\f'
	case 'v':
		char = '\v'
	case 'a':
		char = '\a'
	case 'b':
		if !t.inCharClass() {
			return true, fmt.Errorf("pattern-syntax-error: \\b (word boundary) is not valid XSD 1.0 syntax")
		}
		char = '\b'
	default:
		return false, nil
	}

	if t.inCharClass() {
		if err := t.appendClassEscaped(char, `\`+string(nextChar)); err != nil {
			return true, err
		}
	} else {
		t.writeEscapedLiteral(nextChar)
	}

	t.i += 2
	t.justWroteQuantifier = false
	return true, nil
}

func (t *patternTranslator) handleUnsupportedAnchorEscape(nextChar byte) (bool, error) {
	switch nextChar {
	case 'A', 'Z', 'z', 'B':
		return true, fmt.Errorf("pattern-syntax-error: \\%c is not valid XSD 1.0 syntax (XSD patterns are implicitly anchored)", nextChar)
	default:
		return false, nil
	}
}

func (t *patternTranslator) handleEscapedBackslash(nextChar byte) (bool, error) {
	if nextChar != '\\' {
		return false, nil
	}
	if t.inCharClass() {
		if err := t.appendClassEscaped('\\', `\\`); err != nil {
			return true, err
		}
	} else {
		t.result.WriteString(`\\`)
	}
	t.i += 2
	t.justWroteQuantifier = false
	return true, nil
}

func (t *patternTranslator) handleEscapedMetachar(nextChar byte) (bool, error) {
	switch nextChar {
	case '[', ']', '(', ')', '{', '}', '*', '+', '?', '|', '^', '$', '.':
	default:
		return false, nil
	}

	if t.inCharClass() {
		if err := t.appendClassEscaped(rune(nextChar), `\`+string(nextChar)); err != nil {
			return true, err
		}
	} else {
		t.writeEscapedLiteral(nextChar)
	}
	t.i += 2
	t.justWroteQuantifier = false
	return true, nil
}

func (t *patternTranslator) handleEscapedDash(nextChar byte) (bool, error) {
	if nextChar != '-' {
		return false, nil
	}
	if t.inCharClass() {
		if err := t.appendClassEscaped('-', `\-`); err != nil {
			return true, err
		}
	} else {
		t.result.WriteString(`\-`)
	}
	t.i += 2
	t.justWroteQuantifier = false
	return true, nil
}
