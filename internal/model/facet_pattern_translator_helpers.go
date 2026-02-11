package model

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type outsideMetaHandler func(*patternTranslator) error

var outsideMetaHandlers = map[byte]outsideMetaHandler{
	'^': func(t *patternTranslator) error {
		t.emitOutsideLiteral(`\^`)
		return nil
	},
	'$': func(t *patternTranslator) error {
		t.emitOutsideLiteral(`\$`)
		return nil
	},
	'.': func(t *patternTranslator) error {
		t.emitOutsideLiteral(`[^\n\r]`)
		return nil
	},
	'*': func(t *patternTranslator) error {
		t.emitOutsideQuantifier('*')
		return nil
	},
	'+': func(t *patternTranslator) error {
		t.emitOutsideQuantifier('+')
		return nil
	},
	'?': func(t *patternTranslator) error {
		t.emitOutsideQuantifier('?')
		return nil
	},
	']': func(*patternTranslator) error {
		return fmt.Errorf("pattern-syntax-error: ']' is not valid outside a character class")
	},
}

func (t *patternTranslator) checkLazyQuantifier() error {
	if !t.justWroteQuantifier {
		return nil
	}
	if t.pattern[t.i] == '?' {
		start := max(t.i-2, 0)
		end := min(t.i+1, len(t.pattern))
		return fmt.Errorf("pattern-unsupported: non-greedy quantifier (lazy quantifier) not supported in XSD 1.0 (e.g., %q)", t.pattern[start:end])
	}
	t.justWroteQuantifier = false
	return nil
}

func (t *patternTranslator) handleRepeatQuantifier() (bool, error) {
	if t.pattern[t.i] != '{' {
		return false, nil
	}
	repeatPattern, newPos, err := parseAndValidateRepeat(t.pattern, t.i)
	if err != nil {
		return true, err
	}
	t.result.WriteString(repeatPattern)
	t.i = newPos
	if t.i < len(t.pattern) && t.pattern[t.i] == '?' {
		start := max(t.i-10, 0)
		end := min(t.i+1, len(t.pattern))
		return true, fmt.Errorf("pattern-unsupported: non-greedy quantifier (lazy quantifier) not supported in XSD 1.0 (e.g., %q)", t.pattern[start:end])
	}
	t.justWroteQuantifier = true
	return true, nil
}

func (t *patternTranslator) handleOutsideMeta() (bool, error) {
	handler, ok := outsideMetaHandlers[t.pattern[t.i]]
	if !ok {
		return false, nil
	}
	if err := handler(t); err != nil {
		return true, err
	}
	return true, nil
}

func (t *patternTranslator) handleGroupPrefix() (bool, error) {
	if t.pattern[t.i] != '(' || t.i+1 >= len(t.pattern) || t.pattern[t.i+1] != '?' {
		return false, nil
	}
	end := t.i + 2
	for end < len(t.pattern) && t.pattern[end] != ')' && t.pattern[end] != ':' {
		end++
	}
	modifier := t.pattern[t.i+2 : end]
	return true, fmt.Errorf("pattern-syntax-error: group prefix (?%s) is not valid XSD 1.0 syntax", modifier)
}

func (t *patternTranslator) handleGroupDepth() error {
	if t.pattern[t.i] == '(' {
		t.groupDepth++
		return nil
	}
	if t.pattern[t.i] == ')' {
		if t.groupDepth == 0 {
			return fmt.Errorf("pattern-syntax-error: unbalanced ')' in pattern")
		}
		t.groupDepth--
	}
	return nil
}

func (t *patternTranslator) emitOutsideLiteral(value string) {
	t.result.WriteString(value)
	t.i++
	t.justWroteQuantifier = false
}

func (t *patternTranslator) emitOutsideQuantifier(ch byte) {
	t.result.WriteByte(ch)
	t.i++
	t.justWroteQuantifier = true
}

// translateUnicodePropertyEscape translates \p{...} or \P{...} escapes
func translateUnicodePropertyEscape(pattern string, startIdx int, inCharClass bool) (string, int, error) {
	if startIdx+2 >= len(pattern) || pattern[startIdx+2] != '{' {
		return "", startIdx, fmt.Errorf("pattern-syntax-error: invalid Unicode property escape")
	}

	closeIdx := startIdx + 3
	for closeIdx < len(pattern) && pattern[closeIdx] != '}' {
		closeIdx++
	}
	if closeIdx >= len(pattern) {
		return "", startIdx, fmt.Errorf("pattern-syntax-error: incomplete Unicode property escape")
	}

	propName := pattern[startIdx+3 : closeIdx]

	// reject block-style names (Is... or In...)
	if strings.HasPrefix(propName, "Is") || strings.HasPrefix(propName, "In") {
		return "", startIdx, fmt.Errorf("pattern-unsupported: Unicode block escape %q not supported (Go regexp limitation)", `\p{`+propName+`}`)
	}

	// verify Go supports this property by trying to compile it
	testPattern := `\p{` + propName + `}`
	if inCharClass {
		testPattern = `[` + testPattern + `]`
	}
	if _, err := regexp.Compile(testPattern); err != nil {
		return "", startIdx, fmt.Errorf("pattern-unsupported: Unicode property %q not supported by Go regexp", propName)
	}

	// pass through unchanged (Go supports it)
	negated := pattern[startIdx+1] == 'P'
	if negated {
		return `\P{` + propName + `}`, closeIdx + 1, nil
	}
	return `\p{` + propName + `}`, closeIdx + 1, nil
}

// parseAndValidateRepeat parses a repeat quantifier {m}, {m,}, or {m,n} and validates bounds
func parseAndValidateRepeat(pattern string, startIdx int) (string, int, error) {
	if pattern[startIdx] != '{' {
		return "", startIdx, fmt.Errorf("parseAndValidateRepeat: expected '{'")
	}

	closeIdx := startIdx + 1
	for closeIdx < len(pattern) && pattern[closeIdx] != '}' {
		closeIdx++
	}
	if closeIdx >= len(pattern) {
		return "", startIdx, fmt.Errorf("pattern-syntax-error: unclosed repeat quantifier")
	}

	content := pattern[startIdx+1 : closeIdx]

	// parse {m} or {m,} or {m,n}
	var minCount, maxCount int
	var hasMax bool

	if strings.Contains(content, ",") {
		parts := strings.SplitN(content, ",", 2)
		if len(parts) != 2 {
			return "", startIdx, fmt.Errorf("pattern-syntax-error: invalid repeat quantifier")
		}

		var err error
		minCount, err = strconv.Atoi(TrimXMLWhitespace(parts[0]))
		if err != nil {
			return "", startIdx, fmt.Errorf("pattern-syntax-error: invalid repeat quantifier min value")
		}

		part2 := TrimXMLWhitespace(parts[1])
		if part2 == "" {
			// {m,} - no max
			hasMax = false
		} else {
			// {m,n}
			maxCount, err = strconv.Atoi(part2)
			if err != nil {
				return "", startIdx, fmt.Errorf("pattern-syntax-error: invalid repeat quantifier max value")
			}
			hasMax = true
		}
	} else {
		// {m}
		var err error
		minCount, err = strconv.Atoi(content)
		if err != nil {
			return "", startIdx, fmt.Errorf("pattern-syntax-error: invalid repeat quantifier")
		}
		maxCount = minCount
		hasMax = true
	}

	if minCount < 0 {
		return "", startIdx, fmt.Errorf("pattern-syntax-error: repeat quantifier min must be non-negative")
	}
	if hasMax && maxCount < minCount {
		return "", startIdx, fmt.Errorf("pattern-syntax-error: repeat quantifier max must be >= min")
	}

	if minCount > re2MaxRepeat {
		return "", startIdx, fmt.Errorf("pattern-unsupported: repeat {%d} exceeds RE2 limit of %d", minCount, re2MaxRepeat)
	}
	if hasMax && maxCount > re2MaxRepeat {
		return "", startIdx, fmt.Errorf("pattern-unsupported: repeat {%d,%d} exceeds RE2 limit of %d", minCount, maxCount, re2MaxRepeat)
	}

	// return the original pattern (it's valid)
	return pattern[startIdx : closeIdx+1], closeIdx + 1, nil
}
