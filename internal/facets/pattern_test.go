package facets

import (
	"regexp"
	"strings"
	"testing"
)

func TestTranslateXSDPatternToGo(t *testing.T) {
	tests := []struct {
		name    string
		xsd     string
		wantErr bool
		errMsg  string
		// expected Go pattern (without ^(?:...)$ wrapper)
		re2      string
		matches  []string
		nonMatch []string
	}{
		{
			name:     "empty pattern",
			xsd:      "",
			wantErr:  false,
			re2:      "",
			matches:  []string{""},
			nonMatch: []string{"a", " "},
		},
		{
			name:     "basic literals",
			xsd:      "abc",
			wantErr:  false,
			re2:      "abc",
			matches:  []string{"abc"},
			nonMatch: []string{"ab", "abcd", "ABC"},
		},
		{
			name:     "alternation",
			xsd:      "a|b",
			wantErr:  false,
			re2:      "a|b",
			matches:  []string{"a", "b"},
			nonMatch: []string{"ab", "c"},
		},
		{
			name:     "quantifier",
			xsd:      "a+",
			wantErr:  false,
			re2:      "a+",
			matches:  []string{"a", "aa", "aaa"},
			nonMatch: []string{"", "b"},
		},
		{
			name:     "character class",
			xsd:      "[abc]",
			wantErr:  false,
			re2:      "[abc]",
			matches:  []string{"a", "b", "c"},
			nonMatch: []string{"d", "ab"},
		},
		{
			name:     "digit shorthand",
			xsd:      `\d{3}`,
			wantErr:  false,
			re2:      xsdDigitClass + `{3}`,
			matches:  []string{"123", "456", "000"},
			nonMatch: []string{"12", "1234", "abc"},
		},
		{
			name:     "digit shorthand in character class",
			xsd:      `[\d]`,
			wantErr:  false,
			re2:      xsdDigitClass,
			matches:  []string{"0", "5", "9"},
			nonMatch: []string{"a", " "},
		},
		{
			name:     "non-digit shorthand",
			xsd:      `\D+`,
			wantErr:  false,
			re2:      xsdNotDigitClass + `+`,
			matches:  []string{"abc", "test", "---"},
			nonMatch: []string{"123", "456"},
		},
		{
			name:     "non-digit shorthand in character class",
			xsd:      `[\D]`,
			wantErr:  false,
			re2:      xsdNotDigitClass,
			matches:  []string{"a", "-", " "},
			nonMatch: []string{"0", "5", "9"},
		},
		{
			name:     "non-digit shorthand in negated character class",
			xsd:      `[^\D]`,
			wantErr:  false,
			re2:      xsdDigitClass,
			matches:  []string{"0", "5", "9"},
			nonMatch: []string{"a", "-", " "},
		},
		{
			name:     "whitespace shorthand",
			xsd:      `\s+`,
			wantErr:  false,
			re2:      `[\x20\t\n\r]+`,
			matches:  []string{"   ", "\t\t", "\n\n", " \t\n\r"},
			nonMatch: []string{"abc", ""},
		},
		{
			name:     "whitespace shorthand in character class",
			xsd:      `[\s]`,
			wantErr:  false,
			re2:      `[\x20\t\n\r]`,
			matches:  []string{" ", "\t", "\n", "\r"},
			nonMatch: []string{"a", "ab"},
		},
		{
			name:     "non-whitespace shorthand",
			xsd:      `\S+`,
			wantErr:  false,
			re2:      `[^\x20\t\n\r]+`,
			matches:  []string{"abc", "123", "test"},
			nonMatch: []string{"   ", "a b"},
		},
		{
			name:     "word char shorthand",
			xsd:      `\w+`,
			wantErr:  false,
			re2:      xsdWordClass + `+`,
			matches:  []string{"test123", "abc", "123", "àáâ", "中文"},
			nonMatch: []string{"a-b", "test.test", "   "},
		},
		{
			name:     "non-word char shorthand",
			xsd:      `\W+`,
			wantErr:  false,
			re2:      xsdNotWordClass + `+`,
			matches:  []string{"---", "   ", "!@#"},
			nonMatch: []string{"abc", "123"},
		},
		{
			name:     "non-word char shorthand in character class",
			xsd:      `[\W]`,
			wantErr:  false,
			re2:      xsdNotWordClass,
			matches:  []string{"-", " ", "!"},
			nonMatch: []string{"a", "1"},
		},
		{
			name:     "Unicode property escape",
			xsd:      `\p{Lu}+`,
			wantErr:  false,
			re2:      `\p{Lu}+`,
			matches:  []string{"ABC", "XYZ", "ÀÁÂ"},
			nonMatch: []string{"abc", "123", "aBc"},
		},
		{
			name:     "Unicode property escape negated",
			xsd:      `\P{L}+`,
			wantErr:  false,
			re2:      `\P{L}+`,
			matches:  []string{"123", "!@#", "   "},
			nonMatch: []string{"abc", "ABC", "Test"},
		},
		{
			name:     "escaped anchors",
			xsd:      `^literal$`,
			wantErr:  false,
			re2:      `\^literal\$`,
			matches:  []string{"^literal$"},
			nonMatch: []string{"literal", "^literal", "literal$"},
		},
		{
			name:    "invalid anchor escape \\Z",
			xsd:     `\Z`,
			wantErr: true,
			errMsg:  "not valid XSD 1.0 syntax",
		},
		{
			name:     "dot shorthand",
			xsd:      `.`,
			wantErr:  false,
			re2:      `[^\n\r]`,
			matches:  []string{"a", " ", "1", "\t"},
			nonMatch: []string{"\n", "\r", "\n\r"},
		},
		{
			name:     "counted repeat",
			xsd:      `a{3}`,
			wantErr:  false,
			re2:      `a{3}`,
			matches:  []string{"aaa"},
			nonMatch: []string{"aa", "aaaa"},
		},
		{
			name:     "counted repeat range",
			xsd:      `a{2,4}`,
			wantErr:  false,
			re2:      `a{2,4}`,
			matches:  []string{"aa", "aaa", "aaaa"},
			nonMatch: []string{"a", "aaaaa"},
		},
		{
			name:     "counted repeat unbounded",
			xsd:      `a{2,}`,
			wantErr:  false,
			re2:      `a{2,}`,
			matches:  []string{"aa", "aaa", "aaaa"},
			nonMatch: []string{"a"},
		},
		// rejection tests
		{
			name:    "character class subtraction",
			xsd:     `[A-Z-[AEIOU]]`,
			wantErr: true,
			errMsg:  "character-class subtraction",
		},
		{
			name:    "Unicode block escape",
			xsd:     `\p{IsBasicLatin}`,
			wantErr: true,
			errMsg:  "Unicode block escape",
		},
		{
			name:    "XML NameChar escape \\i",
			xsd:     `\i\c*`,
			wantErr: false,
			re2:     nameStartCharClass + nameCharClass + `*`,
			matches: []string{"a", "_", ":abc", "a1"},
			nonMatch: []string{
				"1",
				" a",
			},
		},
		{
			name:    "XML NameChar escape \\c",
			xsd:     `\c+`,
			wantErr: false,
			re2:     nameCharClass + `+`,
			matches: []string{"a", "a1", "_", ":"},
			nonMatch: []string{
				" ",
				"a b",
			},
		},
		{
			name:    "XML NameChar escape \\I",
			xsd:     `\I\C*`,
			wantErr: false,
			re2:     nameNotStartCharClass + nameNotCharClass + `*`,
			matches: []string{"1", " ", "-"},
			nonMatch: []string{
				"a",
				"_",
			},
		},
		{
			name:    "XML NameChar escape \\C",
			xsd:     `\C+`,
			wantErr: false,
			re2:     nameNotCharClass + `+`,
			matches: []string{" ", "!"},
			nonMatch: []string{
				"a",
				"1",
			},
		},
		{
			name:    "non-digit shorthand in negated class with other content",
			xsd:     `[^\D0]`,
			wantErr: true,
			errMsg:  "negated character class",
		},
		{
			name:     "\\w as only item in character class",
			xsd:      `[\w]`,
			wantErr:  false,
			re2:      xsdWordClass,
			matches:  []string{"a", "Z", "5"},
			nonMatch: []string{" ", "!", "-"},
		},
		{
			name:     "\\w in character class with other content",
			xsd:      `[a\w]`,
			wantErr:  false,
			re2:      `(?:` + xsdWordClass + `|[a])`,
			matches:  []string{"a", "Z", "5"},
			nonMatch: []string{" ", "!", "-"},
		},
		{
			name:     "\\S as only item in character class",
			xsd:      `[\S]`,
			wantErr:  false,
			re2:      `[^\x20\t\n\r]`,
			matches:  []string{"a", "Z", "!"},
			nonMatch: []string{" ", "\t", "\n", "\r"},
		},
		{
			name:     "\\S in character class with other content",
			xsd:      `[a\S]`,
			wantErr:  false,
			re2:      `(?:[^\x20\t\n\r]|[a])`,
			matches:  []string{"a", "Z", "!"},
			nonMatch: []string{" ", "\t", "\n", "\r"},
		},
		{
			name:    "repeat exceeds RE2 limit",
			xsd:     `a{1001}`,
			wantErr: true,
			errMsg:  "exceeds RE2 limit",
		},
		{
			name:    "repeat range exceeds RE2 limit",
			xsd:     `a{500,1500}`,
			wantErr: true,
			errMsg:  "exceeds RE2 limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TranslateXSDPatternToGo(tt.xsd)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TranslateXSDPatternToGo(%q) expected error, got nil", tt.xsd)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("TranslateXSDPatternToGo(%q) error = %q, want error containing %q", tt.xsd, err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("TranslateXSDPatternToGo(%q) unexpected error: %v", tt.xsd, err)
				return
			}

			// check that the pattern is wrapped in ^(?:...)$
			expectedFull := `^(?:` + tt.re2 + `)$`
			if got != expectedFull {
				t.Errorf("TranslateXSDPatternToGo(%q) = %q, want %q", tt.xsd, got, expectedFull)
			}

			// test that the pattern compiles and works
			re, err := regexp.Compile(got)
			if err != nil {
				t.Fatalf("Failed to compile pattern %q: %v", got, err)
			}

			for _, match := range tt.matches {
				if !re.MatchString(match) {
					t.Errorf("Pattern %q should match %q but didn't", got, match)
				}
			}

			for _, nonMatch := range tt.nonMatch {
				if re.MatchString(nonMatch) {
					t.Errorf("Pattern %q should not match %q but did", got, nonMatch)
				}
			}
		})
	}
}

func TestPatternValidateSyntax(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid pattern",
			pattern: `\d{3}`,
			wantErr: false,
		},
		{
			name:    "empty pattern",
			pattern: "",
			wantErr: false,
		},
		{
			name:    "valid Unicode property",
			pattern: `\p{Lu}+`,
			wantErr: false,
		},
		{
			name:    "rejected: character class subtraction",
			pattern: `[A-Z-[AEIOU]]`,
			wantErr: true,
			errMsg:  "character-class subtraction",
		},
		{
			name:    "rejected: Unicode block escape",
			pattern: `\p{IsBasicLatin}`,
			wantErr: true,
			errMsg:  "Unicode block escape",
		},
		{
			name:    "valid: XML NameChar escape",
			pattern: `\i\c*`,
			wantErr: false,
		},
		{
			name:    "valid: \\w as only item in character class",
			pattern: `[\w]`,
			wantErr: false,
		},
		{
			name:    "valid: \\w in character class with other content",
			pattern: `[a\w]`,
			wantErr: false,
		},
		{
			name:    "valid: \\S as only item in character class",
			pattern: `[\S]`,
			wantErr: false,
		},
		{
			name:    "valid: \\S in character class with other content",
			pattern: `[a\S]`,
			wantErr: false,
		},
		{
			name:    "rejected: repeat exceeds limit",
			pattern: `a{1001}`,
			wantErr: true,
			errMsg:  "exceeds RE2 limit",
		},
		{
			name:    "rejected: non-greedy plus quantifier",
			pattern: `a.+?c`,
			wantErr: true,
			errMsg:  "non-greedy quantifier",
		},
		{
			name:    "rejected: non-greedy star quantifier",
			pattern: `a.*?c`,
			wantErr: true,
			errMsg:  "non-greedy quantifier",
		},
		{
			name:    "rejected: non-greedy optional quantifier",
			pattern: `a??b`,
			wantErr: true,
			errMsg:  "non-greedy quantifier",
		},
		{
			name:    "rejected: non-greedy counted repeat",
			pattern: `a.{0,5}?c`,
			wantErr: true,
			errMsg:  "non-greedy quantifier",
		},
		{
			name:    "rejected: non-greedy counted repeat with group",
			pattern: `(a+|b){0,1}?`,
			wantErr: true,
			errMsg:  "non-greedy quantifier",
		},
		{
			name:    "valid: greedy quantifiers",
			pattern: `a.+c`,
			wantErr: false,
		},
		{
			name:    "valid: greedy star",
			pattern: `a.*c`,
			wantErr: false,
		},
		{
			name:    "valid: greedy optional",
			pattern: `a?b`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pattern{Value: tt.pattern}
			err := p.ValidateSyntax()

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSyntax(%q) expected error, got nil", tt.pattern)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSyntax(%q) error = %q, want error containing %q", tt.pattern, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateSyntax(%q) unexpected error: %v", tt.pattern, err)
				}
				// verify regex was compiled
				if p.regex == nil {
					t.Errorf("ValidateSyntax(%q) should have compiled regex, but regex is nil", tt.pattern)
				}
			}
		})
	}
}

func TestPatternValidate(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "matches pattern",
			pattern: `\d{3}`,
			value:   "123",
			wantErr: false,
		},
		{
			name:    "does not match pattern",
			pattern: `\d{3}`,
			value:   "12",
			wantErr: true,
			errMsg:  "does not match pattern",
		},
		{
			name:    "empty pattern matches empty string",
			pattern: "",
			value:   "",
			wantErr: false,
		},
		{
			name:    "empty pattern does not match non-empty",
			pattern: "",
			value:   "a",
			wantErr: true,
		},
		{
			name:    "Unicode property match",
			pattern: `\p{Lu}+`,
			value:   "ABC",
			wantErr: false,
		},
		{
			name:    "Unicode property no match",
			pattern: `\p{Lu}+`,
			value:   "abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pattern{Value: tt.pattern}
			if err := p.ValidateSyntax(); err != nil {
				if !tt.wantErr {
					t.Fatalf("ValidateSyntax(%q) failed: %v", tt.pattern, err)
				}
				return
			}

			tv := &StringTypedValue{Value: tt.value, Typ: nil}
			err := p.Validate(tv, nil)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate(%q) expected error, got nil", tt.value)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate(%q) error = %q, want error containing %q", tt.value, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate(%q) unexpected error: %v", tt.value, err)
				}
			}
		})
	}
}

func TestPatternValidateWithoutValidateSyntax(t *testing.T) {
	p := &Pattern{Value: `\d+`}
	// intentionally skip ValidateSyntax() to test the error message
	tv := &StringTypedValue{Value: "123", Typ: nil}
	err := p.Validate(tv, nil)

	if err == nil {
		t.Fatal("expected error when ValidateSyntax not called, got nil")
	}
	if !strings.Contains(err.Error(), "ValidateSyntax") {
		t.Errorf("error should mention ValidateSyntax, got: %v", err)
	}
}
