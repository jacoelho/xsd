package model

import (
	"fmt"
	"regexp"
	"strings"
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
