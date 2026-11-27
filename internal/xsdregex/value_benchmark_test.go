package xsdregex

import "testing"

// valueCase is one pattern with a value of the length schemas restrict with
// it. The name prefix says the pattern's shape: exact counts ("fixed"), one
// or more bounded counts ("bounded"), and groups or alternation ("group").
type valueCase struct {
	name    string
	pattern string
	value   string
}

var valueCases = []valueCase{
	{"fixed/country", `[A-Z]{2}`, "PT"},
	{"fixed/currency", `[A-Z]{3}`, "EUR"},
	{"fixed/date", `\d{4}-\d{2}-\d{2}`, "2026-11-27"},
	{"fixed/lei", `[A-Z0-9]{18}[0-9]{2}`, "5493001KJTIIGC8Y1R12"},
	{"fixed/uuid", `[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}`, "f81d4fae-7dec-41d0-a765-00a0c91e6bf6"},
	{"bounded/text35", `[0-9a-zA-Z/\-\?:\(\)\.,'\+ ]{1,35}`, "ACME Trading Ltd, 12 High Street"},
	{"bounded/iban", `[A-Z]{2}[0-9]{2}[a-zA-Z0-9]{1,30}`, "DE89370400440532013000"},
	{"bounded/phone", `\+[0-9]{1,3}-[0-9()+\-]{1,30}`, "+44-2079460958"},
	{"bounded/email", `[^@\s]+@[^@\s]+\.[a-zA-Z]{2,}`, "jane.doe@example.org"},
	{"bounded/name-nonascii", `[\p{L}\p{N}\p{Zs},\.\-']{1,70}`, "Müller Straße, Ærø Łódź"},
	{"group/bic", `[A-Z]{6}[A-Z2-9][A-NP-Z0-9]([A-Z0-9]{3})?`, "DEUTDEFF500"},
	{"group/decimal", `-?[0-9]{1,13}(\.[0-9]{1,5})?`, "1234567.89"},
	{"group/language", `[a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*`, "pt-PT-x-example"},
}

func TestValueCasesMatch(t *testing.T) {
	t.Parallel()
	for _, c := range valueCases {
		pattern := mustCompile(t, c.pattern)
		for name, match := range matchers() {
			matched, _, err := match(pattern, c.value, unlimitedWork, nil)
			if err != nil || !matched {
				t.Errorf("%s %s: matched = %v, %v, want true", c.name, name, matched, err)
			}
			matched, _, err = match(pattern, c.value+"\n", unlimitedWork, nil)
			if err != nil || matched {
				t.Errorf("%s %s with a trailing line feed: matched = %v, %v, want false", c.name, name, matched, err)
			}
		}
	}
}

// BenchmarkPatternValues times one match per value through both entry points
// with reused scratch, the way a validation session matches pattern facets.
func BenchmarkPatternValues(b *testing.B) {
	for _, c := range valueCases {
		pattern := benchmarkPattern(b, c.pattern)
		var scratch Scratch
		b.Run(c.name+"/bytes", func(b *testing.B) {
			input := []byte(c.value)
			b.ReportAllocs()
			for b.Loop() {
				if matched, _, err := pattern.MatchBytesWithScratch(input, unlimitedWork, &scratch); err != nil || !matched {
					b.Fatalf("matched = %v, %v", matched, err)
				}
			}
		})
		b.Run(c.name+"/string", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if matched, _, err := pattern.MatchStringWithScratch(c.value, unlimitedWork, &scratch); err != nil || !matched {
					b.Fatalf("matched = %v, %v", matched, err)
				}
			}
		})
	}
}
