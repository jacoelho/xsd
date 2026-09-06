package xsdregex

import (
	"testing"
	"unicode/utf8"
)

// FuzzXSDRegexSyntax exercises the complete parser/compiler contract. Any
// accepted source must produce a bounded immutable pattern; rejected sources
// must be reported as syntax or resource errors without panicking.
func FuzzXSDRegexSyntax(f *testing.F) {
	for _, seed := range []string{
		`[A-Z]{2}\d{4}`,
		`\p{Lu}+`,
		`\p{IsBasicLatin}+`,
		`\i\c*`,
		`[a-z-[aeiou]]+`,
		`a|b`,
		`([a-z]+)?`,
		`\p{}0`,
		`0{0002}`,
		`0{1001,}`,
		`0{1001,1000}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 512 || !utf8.ValidString(source) {
			return
		}
		pattern, err := Compile(source, CompileOptions{})
		if err != nil {
			if !IsSyntax(err) && !IsLimit(err) {
				t.Fatalf("Compile(%q) returned unclassified error %T: %v", source, err, err)
			}
			return
		}
		if pattern == nil {
			t.Fatal("Compile returned nil pattern without error")
		}
		if _, err := pattern.MatchString(""); err != nil && !IsLimit(err) {
			t.Fatalf("MatchString(%q) returned unclassified error %T: %v", source, err, err)
		}
	})
}
