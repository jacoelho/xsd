package xsdregex

import (
	"errors"
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
		pattern, err := Compile(source)
		if err != nil {
			diagnostic, ok := errors.AsType[*Error](err)
			if !ok || (diagnostic.Kind != ErrorSyntax && diagnostic.Kind != ErrorLimit) {
				t.Fatalf("Compile(%q) returned unclassified error %T: %v", source, err, err)
			}
			return
		}
		if pattern == nil {
			t.Fatal("Compile returned nil pattern without error")
		}
		if _, _, err := pattern.MatchStringWithScratch("", unlimitedWork, nil); err != nil && !IsLimit(err) {
			t.Fatalf("MatchString(%q) returned unclassified error %T: %v", source, err, err)
		}
	})
}

// FuzzDFAMatchesFallback compares a DFA with the matcher the same pattern
// compiles to without one, through both entry points.
func FuzzDFAMatchesFallback(f *testing.F) {
	for _, seed := range [][2]string{
		{`[A-Z]{2}\d{4}`, "AB1234"},
		{`(a|ab)*c`, "aababc"},
		{`[\p{L}-[a-z]]+`, "\u00c9\u0080\u07ff\u0800"},
		{`\i\c*`, "x\xff"},
		{`([^a]|ab){1,9}`, "\U0010FFFF\ud7ff\ue000"},
		{`(a?){2,5}b|.`, "\n"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, source, input string) {
		if len(source) > 256 || len(input) > 256 || !utf8.ValidString(source) {
			return
		}
		pattern, err := Compile(source)
		if err != nil || pattern.mode != modeDFA {
			return
		}
		fallback := mustCompileWithoutDFA(t, source)
		for name, match := range matchers() {
			got, work, err := match(pattern, input, unlimitedWork, nil)
			if err != nil {
				t.Fatalf("%s DFA %q on %q: %v", name, source, input, err)
			}
			want, _, err := match(fallback, input, unlimitedWork, nil)
			if err != nil {
				t.Fatalf("%s %s runner %q on %q: %v", name, modeName(fallback), source, input, err)
			}
			if got != want {
				t.Fatalf("%s %q on %q: DFA matched = %v, %s runner matched = %v", name, source, input, got, modeName(fallback), want)
			}
			if limit := 2 * uint64(len(input)); work > limit {
				t.Fatalf("%s %q on %q: DFA work %d exceeds twice the input bytes", name, source, input, work)
			}
		}
	})
}
