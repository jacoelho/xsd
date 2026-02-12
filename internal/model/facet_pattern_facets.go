package model

import (
	"fmt"
	"regexp"
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
		err := p.validateLexical(lexical)
		if err == nil {
			return nil // matched at least one pattern
		}
		lastErr = err
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
