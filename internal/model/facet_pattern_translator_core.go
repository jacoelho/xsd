package model

import (
	"fmt"
	"strings"
)

type charClassState struct {
	lastItem       rune
	lastWasRange   bool
	lastWasDash    bool
	lastItemIsChar bool
	isFirst        bool
}

func (s *charClassState) reset() {
	s.lastItem = 0
	s.lastWasRange = false
	s.lastWasDash = false
	s.lastItemIsChar = false
	s.isFirst = true
}

func (s *charClassState) markNonChar() {
	s.lastWasDash = false
	s.lastWasRange = false
	s.lastItemIsChar = false
	s.isFirst = false
}

func (s *charClassState) handleChar(char rune, classStart int, pattern string) error {
	if s.lastWasDash {
		// this completes a range: lastItem - char
		// validate that the range is valid (start <= end)
		if s.lastItem > char {
			return fmt.Errorf("pattern-syntax-error: invalid range '%c-%c' (start > end) in character class starting at position %d in %q",
				s.lastItem, char, classStart, pattern)
		}
		s.lastWasRange = true
		s.lastWasDash = false
		s.lastItem = char
		s.lastItemIsChar = true
	} else {
		s.lastItem = char
		s.lastWasRange = false
		s.lastItemIsChar = true
	}
	s.isFirst = false
	return nil
}

type patternTranslator struct {
	pattern              string
	classBuf             strings.Builder
	result               strings.Builder
	classDepth           int
	classStart           int
	i                    int
	groupDepth           int
	classState           charClassState
	classNegated         bool
	classHasW            bool
	classHasS            bool
	classHasNotD         bool
	classHasNotNameStart bool
	classHasNotNameChar  bool
	justWroteQuantifier  bool
}

type patternStepHandler func(*patternTranslator) (bool, error)

var (
	charClassStepHandlers = []patternStepHandler{
		(*patternTranslator).handleCharClassEnd,
		(*patternTranslator).handleCharClassSubtraction,
		(*patternTranslator).handleCharClassDash,
	}
	outsideClassStepHandlers = []patternStepHandler{
		(*patternTranslator).handleRepeatQuantifier,
		(*patternTranslator).handleOutsideMeta,
		(*patternTranslator).handleGroupPrefix,
	}
)

func newPatternTranslator(pattern string) *patternTranslator {
	t := &patternTranslator{pattern: pattern}
	t.result.Grow(len(pattern) * 4)
	return t
}

// TranslateXSDPatternToGo translates an XSD 1.0 pattern to Go regexp (RE2) syntax.
// Returns an error for unsupported features (fail-closed approach).
func TranslateXSDPatternToGo(xsdPattern string) (string, error) {
	// empty pattern matches only empty string
	if xsdPattern == "" {
		return `^(?:)$`, nil
	}
	return newPatternTranslator(xsdPattern).translate()
}

func (t *patternTranslator) translate() (string, error) {
	for t.i < len(t.pattern) {
		if handled, err := t.handleEscape(); err != nil {
			return "", err
		} else if handled {
			continue
		}

		if t.classDepth > 0 {
			if handled, err := t.runStepHandlers(charClassStepHandlers); err != nil {
				return "", err
			} else if handled {
				continue
			}
			if err := t.handleCharClassChar(); err != nil {
				return "", err
			}
			continue
		}

		if handled, err := t.handleCharClassStart(); err != nil {
			return "", err
		} else if handled {
			continue
		}

		if err := t.checkLazyQuantifier(); err != nil {
			return "", err
		}

		if handled, err := t.runStepHandlers(outsideClassStepHandlers); err != nil {
			return "", err
		} else if handled {
			continue
		}

		if err := t.handleGroupDepth(); err != nil {
			return "", err
		}

		t.appendLiteralByte(t.pattern[t.i])
	}

	if t.classDepth > 0 {
		return "", fmt.Errorf("pattern-syntax-error: unclosed character class")
	}
	if t.groupDepth > 0 {
		return "", fmt.Errorf("pattern-syntax-error: unclosed '(' in pattern")
	}

	return `^(?:` + t.result.String() + `)$`, nil
}

func (t *patternTranslator) inCharClass() bool {
	return t.classDepth > 0
}

func (t *patternTranslator) runStepHandlers(handlers []patternStepHandler) (bool, error) {
	for _, handler := range handlers {
		handled, err := handler(t)
		if err != nil {
			return true, err
		}
		if handled {
			return true, nil
		}
	}
	return false, nil
}

func (t *patternTranslator) appendLiteralByte(ch byte) {
	t.result.WriteByte(ch)
	t.i++
	t.justWroteQuantifier = false
}

func (t *patternTranslator) appendClassEscaped(char rune, escapeText string) error {
	if err := t.classState.handleChar(char, t.classStart, t.pattern); err != nil {
		return err
	}
	t.classBuf.WriteString(escapeText)
	return nil
}

func (t *patternTranslator) consumeEscape() {
	t.i += 2
	t.justWroteQuantifier = false
}

func (t *patternTranslator) writeEscapedLiteral(ch byte) {
	t.result.WriteByte('\\')
	t.result.WriteByte(ch)
}
