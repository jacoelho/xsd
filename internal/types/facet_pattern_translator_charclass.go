package types

import (
	"fmt"
	"strings"
)

func (t *patternTranslator) handleCharClassStart() (bool, error) {
	if t.pattern[t.i] != '[' {
		return false, nil
	}
	if t.classDepth > 0 {
		return true, fmt.Errorf("pattern-unsupported: nested character classes not supported")
	}

	t.classDepth++
	t.classStart = t.i
	t.classState.reset()
	t.classBuf.Reset()
	t.classNegated = false
	t.classHasW = false
	t.classHasS = false
	t.classHasNotD = false
	t.classHasNotNameStart = false
	t.classHasNotNameChar = false

	t.i++
	if t.i < len(t.pattern) && t.pattern[t.i] == '^' {
		t.classNegated = true
		t.i++
	}
	return true, nil
}

func (t *patternTranslator) handleCharClassEnd() (bool, error) {
	if t.pattern[t.i] != ']' || t.classDepth == 0 {
		return false, nil
	}
	if t.classState.isFirst && !t.classHasW && !t.classHasS && !t.classHasNotD {
		return true, fmt.Errorf("pattern-syntax-error: empty character class")
	}
	classContent := t.classBuf.String()
	if t.classHasNotD && t.classNegated {
		if t.classHasW || t.classHasS || t.classHasNotNameStart || t.classHasNotNameChar || classContent != "" {
			return true, fmt.Errorf("pattern-unsupported: \\D inside negated character class not expressible in RE2")
		}
		t.result.WriteString(xsdDigitClass)
		t.classDepth--
		t.i++
		return true, nil
	}

	if t.classHasW || t.classHasS || t.classHasNotD || t.classHasNotNameStart || t.classHasNotNameChar {
		if t.classNegated {
			return true, fmt.Errorf("pattern-unsupported: negated character class with \\w, \\S, \\I, or \\C is not expressible in RE2")
		}
		var parts []string
		if t.classHasNotD {
			parts = append(parts, xsdNotDigitClass)
		}
		if t.classHasNotNameStart {
			parts = append(parts, nameNotStartCharClass)
		}
		if t.classHasNotNameChar {
			parts = append(parts, nameNotCharClass)
		}
		if t.classHasS {
			parts = append(parts, `[^\x20\t\n\r]`)
		}
		if t.classHasW {
			parts = append(parts, xsdWordClass)
		}
		if classContent != "" {
			parts = append(parts, "["+classContent+"]")
		}
		if len(parts) == 1 {
			t.result.WriteString(parts[0])
		} else {
			t.result.WriteString(`(?:` + strings.Join(parts, "|") + `)`)
		}
	} else {
		if t.classNegated {
			t.result.WriteString(`[^` + classContent + `]`)
		} else {
			t.result.WriteString(`[` + classContent + `]`)
		}
	}
	t.classDepth--
	t.i++
	return true, nil
}

func (t *patternTranslator) handleCharClassSubtraction() (bool, error) {
	if t.classDepth > 0 && t.pattern[t.i] == '-' && t.i+1 < len(t.pattern) && t.pattern[t.i+1] == '[' {
		return true, fmt.Errorf("pattern-unsupported: character-class subtraction (-[) not supported in %q", t.pattern)
	}
	return false, nil
}

func (t *patternTranslator) handleCharClassDash() (bool, error) {
	if t.pattern[t.i] != '-' {
		return false, nil
	}
	if t.classState.isFirst {
		t.classState.lastItem = '-'
		t.classState.lastWasRange = false
		t.classState.lastWasDash = false
		t.classState.lastItemIsChar = true
		t.classState.isFirst = false
		t.classBuf.WriteByte('-')
		t.i++
		return true, nil
	}
	if t.i+1 < len(t.pattern) && t.pattern[t.i+1] == ']' {
		t.classState.lastItem = '-'
		t.classState.lastWasRange = false
		t.classState.lastWasDash = false
		t.classState.lastItemIsChar = true
		t.classState.isFirst = false
		t.classBuf.WriteByte('-')
		t.i++
		return true, nil
	}
	if t.classState.lastWasRange {
		return true, fmt.Errorf("pattern-syntax-error: '-' cannot follow a range in character class at position %d in %q", t.i, t.pattern)
	}
	if t.classState.lastWasDash {
		return true, fmt.Errorf("pattern-syntax-error: consecutive dashes in character class at position %d in %q", t.i, t.pattern)
	}
	if !t.classState.lastItemIsChar {
		return true, fmt.Errorf("pattern-syntax-error: '-' cannot follow a non-character item in character class at position %d in %q", t.i, t.pattern)
	}
	t.classState.lastWasDash = true
	t.classBuf.WriteByte('-')
	t.i++
	return true, nil
}

func (t *patternTranslator) handleCharClassChar() error {
	if t.pattern[t.i] == '[' {
		return fmt.Errorf("pattern-unsupported: nested character classes not supported")
	}
	if err := t.classState.handleChar(rune(t.pattern[t.i]), t.classStart, t.pattern); err != nil {
		return err
	}
	t.classBuf.WriteByte(t.pattern[t.i])
	t.i++
	return nil
}
