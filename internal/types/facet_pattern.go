package types

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// re2MaxRepeat is the maximum repeat count supported by RE2
	re2MaxRepeat = 1000
	// Use Unicode decimal digits (Nd) from Go's regexp tables for XSD \d semantics.
	xsdDigitClassContent = `\p{Nd}`
	xsdDigitClass        = "[" + xsdDigitClassContent + "]"
	xsdNotDigitClass     = "[^" + xsdDigitClassContent + "]"
	xsdWordClass         = `[^\p{P}\p{Z}\p{C}]`
	xsdNotWordClass      = `[\p{P}\p{Z}\p{C}]`
	// XML 1.0 NameStartChar and NameChar ranges (XSD \i and \c).
	nameStartCharClassContent = `:A-Z_a-z` +
		`\x{C0}-\x{D6}\x{D8}-\x{F6}\x{F8}-\x{2FF}\x{370}-\x{37D}\x{37F}-\x{1FFF}` +
		`\x{200C}-\x{200D}\x{2070}-\x{218F}\x{2C00}-\x{2FEF}\x{3001}-\x{D7FF}` +
		`\x{F900}-\x{FDCF}\x{FDF0}-\x{FFFD}\x{10000}-\x{EFFFF}`
	nameCharClassContent = nameStartCharClassContent +
		`\-.\x30-\x39\x{B7}\x{0300}-\x{036F}\x{203F}-\x{2040}`
	nameStartCharClass    = "[" + nameStartCharClassContent + "]"
	nameCharClass         = "[" + nameCharClassContent + "]"
	nameNotStartCharClass = "[^" + nameStartCharClassContent + "]"
	nameNotCharClass      = "[^" + nameCharClassContent + "]"
)

// Pattern represents a pattern facet (regex)
type Pattern struct {
	regex     *regexp.Regexp
	Value     string
	GoPattern string
}

// Name returns the facet name
func (p *Pattern) Name() string {
	return "pattern"
}

// ValidateSyntax validates that the pattern value is a valid XSD regex pattern
// and translates it to Go regex. This should be called during schema schemacheck.
func (p *Pattern) ValidateSyntax() error {
	// empty pattern is valid per XSD spec (matches only empty string)
	if p.Value == "" {
		// empty pattern translates to ^(?:)$
		goPattern, err := TranslateXSDPatternToGo("")
		if err != nil {
			return fmt.Errorf("pattern facet: %w", err)
		}
		p.GoPattern = goPattern
		regex, err := regexp.Compile(goPattern)
		if err != nil {
			return fmt.Errorf("pattern facet: failed to compile empty pattern: %w", err)
		}
		p.regex = regex
		return nil
	}

	// translate XSD pattern to Go regex
	goPattern, err := TranslateXSDPatternToGo(p.Value)
	if err != nil {
		return fmt.Errorf("pattern facet: %w", err)
	}
	p.GoPattern = goPattern

	regex, err := regexp.Compile(goPattern)
	if err != nil {
		return fmt.Errorf("pattern facet: failed to compile pattern '%s': %w", p.Value, err)
	}
	p.regex = regex

	return nil
}

// Validate checks if the value matches the pattern
func (p *Pattern) Validate(value TypedValue, _ Type) error {
	return p.ValidateLexical(value.Lexical(), nil)
}

// ValidateLexical validates a lexical value against the pattern.
func (p *Pattern) ValidateLexical(lexical string, _ Type) error {
	return p.validateLexical(lexical)
}

// validateLexical validates a lexical string value against the pattern.
// Requires ValidateSyntax() to have been called first.
func (p *Pattern) validateLexical(lexical string) error {
	if p.regex == nil {
		return fmt.Errorf("pattern not compiled: ValidateSyntax() must be called before Validate()")
	}

	if !p.regex.MatchString(lexical) {
		return fmt.Errorf("does not match pattern '%s'", p.Value)
	}
	return nil
}

// PatternSet groups multiple patterns from the same derivation step.
// Per XSD spec, patterns from the same step are ORed together.
type PatternSet struct {
	// All patterns from the same derivation step
	Patterns []*Pattern
}

// Name returns the facet name
func (ps *PatternSet) Name() string {
	return "pattern"
}

// ValidateSyntax validates all patterns in the set
func (ps *PatternSet) ValidateSyntax() error {
	for _, p := range ps.Patterns {
		if err := p.ValidateSyntax(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks if the value matches ANY pattern in the set (OR semantics)
func (ps *PatternSet) Validate(value TypedValue, _ Type) error {
	return ps.ValidateLexical(value.Lexical(), nil)
}

// ValidateLexical validates a lexical value against a pattern set.
func (ps *PatternSet) ValidateLexical(lexical string, _ Type) error {
	if len(ps.Patterns) == 0 {
		return nil
	}

	// value must match at least one pattern (OR)
	var lastErr error
	for _, p := range ps.Patterns {
		if err := p.validateLexical(lexical); err == nil {
			return nil // matched at least one pattern
		} else {
			lastErr = err
		}
	}

	// none matched - return an error listing all patterns
	if len(ps.Patterns) == 1 {
		return lastErr
	}

	var patterns []string
	for _, p := range ps.Patterns {
		patterns = append(patterns, "'"+p.Value+"'")
	}
	return fmt.Errorf("does not match any pattern in set: %s", strings.Join(patterns, ", "))
}

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
	pattern string
	i       int
	result  strings.Builder

	classDepth int
	classStart int
	classState charClassState

	classBuf             strings.Builder
	classNegated         bool
	classHasW            bool
	classHasS            bool
	classHasNotD         bool
	classHasNotNameStart bool
	classHasNotNameChar  bool

	groupDepth          int
	justWroteQuantifier bool
}

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
		if handled, err := t.handleEscape(); handled {
			if err != nil {
				return "", err
			}
			continue
		}

		if t.classDepth > 0 {
			if handled, err := t.handleCharClassEnd(); handled {
				if err != nil {
					return "", err
				}
				continue
			}
			if handled, err := t.handleCharClassSubtraction(); handled {
				if err != nil {
					return "", err
				}
				continue
			}
			if handled, err := t.handleCharClassDash(); handled {
				if err != nil {
					return "", err
				}
				continue
			}
			if _, err := t.handleCharClassChar(); err != nil {
				return "", err
			}
			continue
		}

		if handled, err := t.handleCharClassStart(); handled {
			if err != nil {
				return "", err
			}
			continue
		}

		if err := t.checkLazyQuantifier(); err != nil {
			return "", err
		}

		if handled, err := t.handleRepeatQuantifier(); handled {
			if err != nil {
				return "", err
			}
			continue
		}

		if handled, err := t.handleOutsideMeta(); handled {
			if err != nil {
				return "", err
			}
			continue
		}

		if handled, err := t.handleGroupPrefix(); handled {
			if err != nil {
				return "", err
			}
			continue
		}

		if err := t.handleGroupDepth(); err != nil {
			return "", err
		}

		t.result.WriteByte(t.pattern[t.i])
		t.i++
		t.justWroteQuantifier = false
	}

	if t.classDepth > 0 {
		return "", fmt.Errorf("pattern-syntax-error: unclosed character class")
	}
	if t.groupDepth > 0 {
		return "", fmt.Errorf("pattern-syntax-error: unclosed '(' in pattern")
	}

	return `^(?:` + t.result.String() + `)$`, nil
}

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

	if handled, err := t.handleNameEscape(nextChar); handled {
		return true, err
	}
	if handled, err := t.handleDigitEscape(nextChar); handled {
		return true, err
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

func (t *patternTranslator) handleNameEscape(nextChar byte) (bool, error) {
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
		return false, nil
	}

	t.i += 2
	t.justWroteQuantifier = false
	return true, nil
}

func (t *patternTranslator) handleDigitEscape(nextChar byte) (bool, error) {
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
		return false, nil
	}

	t.i += 2
	t.justWroteQuantifier = false
	return true, nil
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

func (t *patternTranslator) handleCharClassChar() (bool, error) {
	if t.pattern[t.i] == '[' {
		return true, fmt.Errorf("pattern-unsupported: nested character classes not supported")
	}
	if err := t.classState.handleChar(rune(t.pattern[t.i]), t.classStart, t.pattern); err != nil {
		return true, err
	}
	t.classBuf.WriteByte(t.pattern[t.i])
	t.i++
	return true, nil
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
	switch t.pattern[t.i] {
	case '^':
		t.result.WriteString(`\^`)
		t.i++
		t.justWroteQuantifier = false
		return true, nil
	case '$':
		t.result.WriteString(`\$`)
		t.i++
		t.justWroteQuantifier = false
		return true, nil
	case ']':
		return true, fmt.Errorf("pattern-syntax-error: ']' is not valid outside a character class")
	case '.':
		t.result.WriteString(`[^\n\r]`)
		t.i++
		t.justWroteQuantifier = false
		return true, nil
	case '*':
		t.result.WriteByte('*')
		t.i++
		t.justWroteQuantifier = true
		return true, nil
	case '+':
		t.result.WriteByte('+')
		t.i++
		t.justWroteQuantifier = true
		return true, nil
	case '?':
		t.result.WriteByte('?')
		t.i++
		t.justWroteQuantifier = true
		return true, nil
	default:
		return false, nil
	}
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

func (t *patternTranslator) inCharClass() bool {
	return t.classDepth > 0
}

func (t *patternTranslator) appendClassEscaped(char rune, escapeText string) error {
	if err := t.classState.handleChar(char, t.classStart, t.pattern); err != nil {
		return err
	}
	t.classBuf.WriteString(escapeText)
	return nil
}

func (t *patternTranslator) writeEscapedLiteral(ch byte) {
	t.result.WriteByte('\\')
	t.result.WriteByte(ch)
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
	var min, max int
	var hasMax bool

	if strings.Contains(content, ",") {
		parts := strings.SplitN(content, ",", 2)
		if len(parts) != 2 {
			return "", startIdx, fmt.Errorf("pattern-syntax-error: invalid repeat quantifier")
		}

		var err error
		min, err = strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return "", startIdx, fmt.Errorf("pattern-syntax-error: invalid repeat quantifier min value")
		}

		part2 := strings.TrimSpace(parts[1])
		if part2 == "" {
			// {m,} - no max
			hasMax = false
		} else {
			// {m,n}
			max, err = strconv.Atoi(part2)
			if err != nil {
				return "", startIdx, fmt.Errorf("pattern-syntax-error: invalid repeat quantifier max value")
			}
			hasMax = true
		}
	} else {
		// {m}
		var err error
		min, err = strconv.Atoi(content)
		if err != nil {
			return "", startIdx, fmt.Errorf("pattern-syntax-error: invalid repeat quantifier")
		}
		max = min
		hasMax = true
	}

	if min < 0 {
		return "", startIdx, fmt.Errorf("pattern-syntax-error: repeat quantifier min must be non-negative")
	}
	if hasMax && max < min {
		return "", startIdx, fmt.Errorf("pattern-syntax-error: repeat quantifier max must be >= min")
	}

	if min > re2MaxRepeat {
		return "", startIdx, fmt.Errorf("pattern-unsupported: repeat {%d} exceeds RE2 limit of %d", min, re2MaxRepeat)
	}
	if hasMax && max > re2MaxRepeat {
		return "", startIdx, fmt.Errorf("pattern-unsupported: repeat {%d,%d} exceeds RE2 limit of %d", min, max, re2MaxRepeat)
	}

	// return the original pattern (it's valid)
	return pattern[startIdx : closeIdx+1], closeIdx + 1, nil
}
