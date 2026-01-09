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
	// Original XSD pattern (for error messages)
	Value string
	// Translated Go regex pattern
	GoPattern string
	// Compiled regex (set during ValidateSyntax)
	regex *regexp.Regexp
}

// Name returns the facet name
func (p *Pattern) Name() string {
	return "pattern"
}

// ValidateSyntax validates that the pattern value is a valid XSD regex pattern
// and translates it to Go regex. This should be called during schema validation.
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
	lexical := value.Lexical()
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
	if len(ps.Patterns) == 0 {
		return nil
	}

	lexical := value.Lexical()

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

// TranslateXSDPatternToGo translates an XSD 1.0 pattern to Go regexp (RE2) syntax.
// Returns an error for unsupported features (fail-closed approach).
func TranslateXSDPatternToGo(xsdPattern string) (string, error) {
	// empty pattern matches only empty string
	if xsdPattern == "" {
		return `^(?:)$`, nil
	}

	var result strings.Builder
	result.Grow(len(xsdPattern) * 4)

	i := 0
	charClassDepth := 0 // track nested character class depth (for proper parsing)

	// for character class validation: track the last character/range endpoint
	var charClassStart int     // position where current char class started
	var lastCharClassItem rune // last character added to the class (for range validation)
	var lastWasRange bool      // was the last item a range (so next - must be literal or start new range)
	var lastWasDash bool       // was the last character a dash (for detecting invalid patterns)
	lastItemIsChar := false    // was the last item a single character (ranges only apply then)
	isFirstInClass := false    // is this the first item after [ or [^

	var classBuf strings.Builder
	classNegated := false
	classHasW := false
	classHasS := false
	classHasNotD := false
	classHasNotNameStart := false
	classHasNotNameChar := false
	groupDepth := 0

	// track if we just wrote a quantifier (to detect non-greedy quantifiers)
	justWroteQuantifier := false

	for i < len(xsdPattern) {
		char := xsdPattern[i]

		if char == '\\' {
			if i+1 >= len(xsdPattern) {
				return "", fmt.Errorf("pattern-syntax-error: escape sequence at end of pattern")
			}
			nextChar := xsdPattern[i+1]

			// handle Unicode escapes: reject \u (not valid XSD syntax)
			if nextChar == 'u' {
				return "", fmt.Errorf("pattern-syntax-error: \\u escape is not valid XSD 1.0 syntax (use XML character reference &#x; instead)")
			}

			// handle Unicode property escapes: \p{...} or \P{...}
			if nextChar == 'p' || nextChar == 'P' {
				translated, newIdx, err := translateUnicodePropertyEscape(xsdPattern, i, charClassDepth > 0)
				if err != nil {
					return "", err
				}
				if charClassDepth > 0 {
					classBuf.WriteString(translated)
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
				} else {
					result.WriteString(translated)
				}
				i = newIdx
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue
			}

			// handle XSD-specific escapes
			switch nextChar {
			case 'i':
				// XML NameStartChar escape
				if charClassDepth > 0 {
					classBuf.WriteString(nameStartCharClassContent)
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
				} else {
					result.WriteString(nameStartCharClass)
				}
				i += 2
				justWroteQuantifier = false
				continue
			case 'I':
				// negated XML NameStartChar escape
				if charClassDepth > 0 {
					classHasNotNameStart = true
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
				} else {
					result.WriteString(nameNotStartCharClass)
				}
				i += 2
				justWroteQuantifier = false
				continue
			case 'c':
				// XML NameChar escape
				if charClassDepth > 0 {
					classBuf.WriteString(nameCharClassContent)
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
				} else {
					result.WriteString(nameCharClass)
				}
				i += 2
				justWroteQuantifier = false
				continue
			case 'C':
				// negated XML NameChar escape
				if charClassDepth > 0 {
					classHasNotNameChar = true
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
				} else {
					result.WriteString(nameNotCharClass)
				}
				i += 2
				justWroteQuantifier = false
				continue

			case 'd':
				// digit shorthand - Unicode decimal digits
				if charClassDepth > 0 {
					classBuf.WriteString(xsdDigitClassContent)
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
				} else {
					result.WriteString(xsdDigitClass)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'D':
				// non-digit shorthand
				if charClassDepth > 0 {
					classHasNotD = true
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
				} else {
					result.WriteString(xsdNotDigitClass)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 's':
				// whitespace shorthand - XSD defines exactly these 4 chars
				if charClassDepth > 0 {
					classBuf.WriteString(`\x20\t\n\r`)
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
				} else {
					result.WriteString(`[\x20\t\n\r]`)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'S':
				// non-whitespace shorthand
				if charClassDepth > 0 {
					if classNegated {
						return "", fmt.Errorf("pattern-unsupported: \\S inside negated character class not expressible in RE2")
					}
					classHasS = true
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
					i += 2
					justWroteQuantifier = false
					continue
				}
				result.WriteString(`[^\x20\t\n\r]`)
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'w':
				// word character shorthand
				if charClassDepth > 0 {
					if classNegated {
						return "", fmt.Errorf("pattern-unsupported: \\w inside negated character class not expressible in RE2")
					}
					classHasW = true
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
					i += 2
					justWroteQuantifier = false
					continue
				}
				result.WriteString(xsdWordClass)
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'W':
				// non-word character shorthand
				if charClassDepth > 0 {
					classBuf.WriteString(`\p{P}\p{Z}\p{C}`)
					lastWasDash = false
					lastWasRange = false
					lastItemIsChar = false
					isFirstInClass = false
				} else {
					result.WriteString(xsdNotWordClass)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'n':
				if charClassDepth > 0 {
					if err := handleCharClassChar('\n', &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteByte('\\')
					classBuf.WriteByte(nextChar)
				} else {
					result.WriteByte('\\')
					result.WriteByte(nextChar)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'r':
				if charClassDepth > 0 {
					if err := handleCharClassChar('\r', &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteByte('\\')
					classBuf.WriteByte(nextChar)
				} else {
					result.WriteByte('\\')
					result.WriteByte(nextChar)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 't':
				if charClassDepth > 0 {
					if err := handleCharClassChar('\t', &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteByte('\\')
					classBuf.WriteByte(nextChar)
				} else {
					result.WriteByte('\\')
					result.WriteByte(nextChar)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'f':
				if charClassDepth > 0 {
					if err := handleCharClassChar('\f', &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteByte('\\')
					classBuf.WriteByte(nextChar)
				} else {
					result.WriteByte('\\')
					result.WriteByte(nextChar)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'v':
				if charClassDepth > 0 {
					if err := handleCharClassChar('\v', &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteByte('\\')
					classBuf.WriteByte(nextChar)
				} else {
					result.WriteByte('\\')
					result.WriteByte(nextChar)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'a':
				if charClassDepth > 0 {
					if err := handleCharClassChar('\a', &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteByte('\\')
					classBuf.WriteByte(nextChar)
				} else {
					result.WriteByte('\\')
					result.WriteByte(nextChar)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'b':
				// \b is only valid inside character class (backspace)
				// outside character class, it's NOT valid XSD syntax (no word boundary in XSD)
				if charClassDepth > 0 {
					if err := handleCharClassChar('\b', &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteByte('\\')
					classBuf.WriteByte('b')
				} else {
					return "", fmt.Errorf("pattern-syntax-error: \\b (word boundary) is not valid XSD 1.0 syntax")
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case 'A', 'Z', 'z', 'B':
				// XSD does not support these anchor escapes:
				// \A - beginning of string (Perl/PCRE)
				// \Z - end of string before newline (Perl/PCRE)
				// \z - end of string (Perl/PCRE)
				// \B - non-word boundary (Perl/PCRE)
				return "", fmt.Errorf("pattern-syntax-error: \\%c is not valid XSD 1.0 syntax (XSD patterns are implicitly anchored)", nextChar)

			case '\\':
				// escaped backslash - can be range endpoint
				if charClassDepth > 0 {
					if err := handleCharClassChar('\\', &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteString(`\\`)
				} else {
					result.WriteString(`\\`)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case '[', ']', '(', ')', '{', '}', '*', '+', '?', '|', '^', '$', '.':
				// escaped metacharacters - pass through
				if charClassDepth > 0 {
					if err := handleCharClassChar(rune(nextChar), &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteByte('\\')
					classBuf.WriteByte(nextChar)
				} else {
					result.WriteByte('\\')
					result.WriteByte(nextChar)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			case '-':
				// escaped dash - literal dash character
				if charClassDepth > 0 {
					if err := handleCharClassChar('-', &lastCharClassItem, &lastWasRange, &lastWasDash, &lastItemIsChar, &isFirstInClass, charClassStart, xsdPattern); err != nil {
						return "", err
					}
					classBuf.WriteString(`\-`)
				} else {
					result.WriteString(`\-`)
				}
				i += 2
				justWroteQuantifier = false // escape sequences are not quantifiers
				continue

			default:
				// XSD only allows specific escape sequences. Unknown escapes are syntax errors.
				// valid escapes: \n, \r, \t, \d, \D, \s, \S, \w, \W, \p{}, \P{}, \i, \I, \c, \C,
				// and escaped metacharacters: \\, \[, \], \(, \), \{, \}, \*, \+, \?, \|, \^, \$, \., \-
				// digit escapes (\0-\9) could be backreferences but XSD doesn't support them
				if nextChar >= '0' && nextChar <= '9' {
					return "", fmt.Errorf("pattern-syntax-error: \\%c backreference is not valid XSD 1.0 syntax", nextChar)
				}
				return "", fmt.Errorf("pattern-syntax-error: \\%c is not a valid XSD 1.0 escape sequence", nextChar)
			}
		}

		// handle character class boundaries (only for non-escaped [ and ])
		if char == '[' {
			if charClassDepth > 0 {
				return "", fmt.Errorf("pattern-unsupported: nested character classes not supported")
			}
			charClassDepth++
			charClassStart = i
			lastCharClassItem = 0
			lastWasRange = false
			lastWasDash = false
			lastItemIsChar = false
			isFirstInClass = true
			classBuf.Reset()
			classNegated = false
			classHasW = false
			classHasS = false
			classHasNotD = false
			classHasNotNameStart = false
			classHasNotNameChar = false
			i++
			if i < len(xsdPattern) && xsdPattern[i] == '^' {
				classNegated = true
				i++
			}
			continue
		}

		if char == ']' && charClassDepth > 0 {
			if isFirstInClass && !classHasW && !classHasS && !classHasNotD {
				return "", fmt.Errorf("pattern-syntax-error: empty character class")
			}
			classContent := classBuf.String()
			if classHasNotD && classNegated {
				if classHasW || classHasS || classHasNotNameStart || classHasNotNameChar || classContent != "" {
					return "", fmt.Errorf("pattern-unsupported: \\D inside negated character class not expressible in RE2")
				}
				result.WriteString(xsdDigitClass)
				charClassDepth--
				i++
				continue
			}

			if classHasW || classHasS || classHasNotD || classHasNotNameStart || classHasNotNameChar {
				if classNegated {
					return "", fmt.Errorf("pattern-unsupported: negated character class with \\w, \\S, \\I, or \\C is not expressible in RE2")
				}
				var parts []string
				if classHasNotD {
					parts = append(parts, xsdNotDigitClass)
				}
				if classHasNotNameStart {
					parts = append(parts, nameNotStartCharClass)
				}
				if classHasNotNameChar {
					parts = append(parts, nameNotCharClass)
				}
				if classHasS {
					parts = append(parts, `[^\x20\t\n\r]`)
				}
				if classHasW {
					parts = append(parts, xsdWordClass)
				}
				if classContent != "" {
					parts = append(parts, "["+classContent+"]")
				}
				if len(parts) == 1 {
					result.WriteString(parts[0])
				} else {
					result.WriteString(`(?:` + strings.Join(parts, "|") + `)`)
				}
			} else {
				if classNegated {
					result.WriteString(`[^` + classContent + `]`)
				} else {
					result.WriteString(`[` + classContent + `]`)
				}
			}
			charClassDepth--
			i++
			continue
		}

		// check for character class subtraction: -[...]
		if charClassDepth > 0 && char == '-' && i+1 < len(xsdPattern) && xsdPattern[i+1] == '[' {
			return "", fmt.Errorf("pattern-unsupported: character-class subtraction (-[) not supported in %q", xsdPattern)
		}

		if charClassDepth > 0 && char == '-' {
			// dash at the very start of class (after [ or [^) is literal
			if isFirstInClass {
				lastCharClassItem = '-'
				lastWasRange = false
				lastWasDash = false
				lastItemIsChar = true
				isFirstInClass = false
				classBuf.WriteByte('-')
				i++
				continue
			}
			// dash at the very end of class is literal
			if i+1 < len(xsdPattern) && xsdPattern[i+1] == ']' {
				lastCharClassItem = '-'
				lastWasRange = false
				lastWasDash = false
				lastItemIsChar = true
				isFirstInClass = false
				classBuf.WriteByte('-')
				i++
				continue
			}
			// dash immediately after a completed range is not allowed in XSD 1.0.
			if lastWasRange {
				return "", fmt.Errorf("pattern-syntax-error: '-' cannot follow a range in character class at position %d in %q", i, xsdPattern)
			}
			// dash after another dash is invalid
			if lastWasDash {
				return "", fmt.Errorf("pattern-syntax-error: consecutive dashes in character class at position %d in %q", i, xsdPattern)
			}
			// dash after a non-character item is invalid (can't start a range).
			if !lastItemIsChar {
				return "", fmt.Errorf("pattern-syntax-error: '-' cannot follow a non-character item in character class at position %d in %q", i, xsdPattern)
			}
			// otherwise, dash after a single character starts a potential range
			lastWasDash = true
			classBuf.WriteByte('-')
			i++
			continue
		}

		if charClassDepth > 0 {
			currentChar := rune(char)
			if lastWasDash {
				// this completes a range: lastCharClassItem - currentChar
				// validate that the range is valid (start <= end)
				if lastCharClassItem > currentChar {
					return "", fmt.Errorf("pattern-syntax-error: invalid range '%c-%c' (start > end) in character class starting at position %d in %q",
						lastCharClassItem, currentChar, charClassStart, xsdPattern)
				}
				lastWasRange = true
				lastWasDash = false
				lastCharClassItem = currentChar
				lastItemIsChar = true
			} else {
				lastCharClassItem = currentChar
				lastWasRange = false
				lastItemIsChar = true
			}
			isFirstInClass = false
			classBuf.WriteByte(char)
			i++
			continue
		}

		// check for non-greedy quantifier: ? after +, *, ?, or {m,n}
		if justWroteQuantifier && char == '?' {
			start := max(i-2, 0)
			end := min(i+1, len(xsdPattern))
			return "", fmt.Errorf("pattern-unsupported: non-greedy quantifier (lazy quantifier) not supported in XSD 1.0 (e.g., %q)", xsdPattern[start:end])
		}
		justWroteQuantifier = false

		// handle quantifiers and validate repeat counts (outside character classes)
		if charClassDepth == 0 && char == '{' {
			// check for counted repeat: {m} or {m,} or {m,n}
			repeatPattern, newPos, err := parseAndValidateRepeat(xsdPattern, i)
			if err != nil {
				return "", err
			}
			result.WriteString(repeatPattern)
			i = newPos
			// check if next character is ? (non-greedy quantifier)
			if i < len(xsdPattern) && xsdPattern[i] == '?' {
				start := max(i-10, 0)
				end := min(i+1, len(xsdPattern))
				return "", fmt.Errorf("pattern-unsupported: non-greedy quantifier (lazy quantifier) not supported in XSD 1.0 (e.g., %q)", xsdPattern[start:end])
			}
			justWroteQuantifier = true
			continue
		}

		if charClassDepth == 0 {
			switch char {
			case '^':
				// ^ is literal in XSD, but anchor in Go - escape it
				result.WriteString(`\^`)
				i++
				justWroteQuantifier = false
				continue
			case '$':
				// $ is literal in XSD, but anchor in Go - escape it
				result.WriteString(`\$`)
				i++
				justWroteQuantifier = false
				continue
			case ']':
				// ']' is only valid to close a character class in XSD.
				return "", fmt.Errorf("pattern-syntax-error: ']' is not valid outside a character class")
			case '.':
				// . in XSD matches any char except \n and \r
				result.WriteString(`[^\n\r]`)
				i++
				justWroteQuantifier = false
				continue
			case '*':
				// kleene star quantifier
				result.WriteByte('*')
				i++
				justWroteQuantifier = true
				continue
			case '+':
				// one-or-more quantifier
				result.WriteByte('+')
				i++
				justWroteQuantifier = true
				continue
			case '?':
				// zero-or-one quantifier (but check if it's after another quantifier first)
				if justWroteQuantifier {
					start := max(i-2, 0)
					end := min(i+1, len(xsdPattern))
					return "", fmt.Errorf("pattern-unsupported: non-greedy quantifier (lazy quantifier) not supported in XSD 1.0 (e.g., %q)", xsdPattern[start:end])
				}
				result.WriteByte('?')
				i++
				justWroteQuantifier = true
				continue
			}
		}

		// XSD 1.0 does not support Perl-style group prefixes like (?:...) or (?m).
		if charClassDepth == 0 && char == '(' && i+1 < len(xsdPattern) && xsdPattern[i+1] == '?' {
			end := i + 2
			for end < len(xsdPattern) && xsdPattern[end] != ')' && xsdPattern[end] != ':' {
				end++
			}
			modifier := xsdPattern[i+2 : end]
			return "", fmt.Errorf("pattern-syntax-error: group prefix (?%s) is not valid XSD 1.0 syntax", modifier)
		}

		if charClassDepth == 0 && char == '(' {
			groupDepth++
		}
		if charClassDepth == 0 && char == ')' {
			if groupDepth == 0 {
				return "", fmt.Errorf("pattern-syntax-error: unbalanced ')' in pattern")
			}
			groupDepth--
		}

		result.WriteByte(char)
		i++
		justWroteQuantifier = false
	}

	if charClassDepth > 0 {
		return "", fmt.Errorf("pattern-syntax-error: unclosed character class")
	}
	if groupDepth > 0 {
		return "", fmt.Errorf("pattern-syntax-error: unclosed '(' in pattern")
	}

	// wrap in ^(?:...)$ for whole-string matching
	translated := result.String()
	return `^(?:` + translated + `)$`, nil
}

// handleCharClassChar handles a character inside a character class for range validation
func handleCharClassChar(char rune, lastItem *rune, lastWasRange *bool, lastWasDash *bool, lastItemIsChar *bool, isFirst *bool, classStart int, pattern string) error {
	if *lastWasDash {
		// this completes a range: lastItem - char
		// validate that the range is valid (start <= end)
		if *lastItem > char {
			return fmt.Errorf("pattern-syntax-error: invalid range '%c-%c' (start > end) in character class starting at position %d in %q",
				*lastItem, char, classStart, pattern)
		}
		*lastWasRange = true
		*lastWasDash = false
		*lastItem = char
		*lastItemIsChar = true
	} else {
		*lastItem = char
		*lastWasRange = false
		*lastItemIsChar = true
	}
	*isFirst = false
	return nil
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
