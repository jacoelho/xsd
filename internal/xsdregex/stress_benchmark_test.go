package xsdregex

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// stressCase is one pattern and input family. The name prefix says which
// matcher cost it exercises: per-character class tests ("class"), the number
// of states active per character ("states"), counted-repeat expansion
// ("repeat"), inputs that drive backtracking matchers to super-linear time
// ("backtrack"), and inputs rejected before their end ("reject").
type stressCase struct {
	name    string
	pattern string
	unit    string
	tail    string
	match   bool
}

const (
	stressASCIIText    = "ACME Trading Ltd, 12 High Street London "
	stressNonASCIIText = "Müller Straße Ærø Łódź Ćirić Ñandú "
	stressASCIIName    = "ACME_Trading-Ltd.12"
	stressNonASCIIName = "Müller_Straße-Łódź.Ñandú"
	stressCJKText      = "数据交换格式 2024 "
)

// input repeats unit to runes characters, the last of which is tail when set.
func (c stressCase) input(runes int) string {
	body := runes - utf8.RuneCountInString(c.tail)
	return string([]rune(strings.Repeat(c.unit, body))[:body]) + c.tail
}

var stressCases = []stressCase{
	{"class/finx-ascii", `[0-9a-zA-Z/\-\?:\(\)\.,'\+ ]+`, stressASCIIText, "", true},
	{"class/unicode-categories-ascii", `[\p{L}\p{N}\p{Zs},]+`, stressASCIIText, "", true},
	{"class/unicode-categories-nonascii", `[\p{L}\p{N}\p{Zs},]+`, stressNonASCIIText, "", true},
	{"class/ncname-ascii", `[\i-[:]][\c-[:]]*`, stressASCIIName, "", true},
	{"class/ncname-nonascii", `[\i-[:]][\c-[:]]*`, stressNonASCIIName, "", true},
	{"class/unicode-categories-cjk", `[\p{L}\p{N}\s]+`, stressCJKText, "", true},
	{"states/whitespace-token", `(\s*[^\s]\s*)+`, stressASCIIText, "", true},
	{"states/fin-restricted", `([0-9a-zA-Z\-\?:\(\)\.,'\+ ]([0-9a-zA-Z\-\?:\(\)\.,'\+ ]*(/[0-9a-zA-Z\-\?:\(\)\.,'\+ ])?)*)`, stressASCIIText, "", true},
	{"states/overlapping-alternation", `([a-z]|[a-z][a-z]|[a-z][a-z][a-z])*`, "abcdefghij", "", true},
	{"states/deterministic-unrolled", strings.Repeat(`(a|b)c`, 16) + `[abc]*`, "ac", "", true},
	{"states/wide-general", `(a|b)*a` + strings.Repeat(`(a|b)`, 20), "ab", "", false},
	{"repeat/bounded-class", `[0-9a-zA-Z/\-\?:\(\)\.\n\r,'\+ ]{1,2048}`, stressASCIIText, "", true},
	{"repeat/bounded-alternation", `([A-Za-z0-9]|[\+\?/\-:\(\)\., ]){1,2048}`, stressASCIIText, "", true},
	{"repeat/bounded-group", `([a-z]+[0-9]){1,512}`, "abc1", "", true},
	{"repeat/nullable-body", `([a-z]*[0-9]?){1,512}`, "abc1", "", true},
	{"repeat/ambiguous-optional", `([a-z]+){1,512}`, "abcdefghij", "", true},
	{"repeat/ambiguous-required", `(a|aa){32,512}`, "a", "", true},
	{"backtrack/nested-star", `(a*)*b`, "a", "", false},
	{"backtrack/overlapping-alternation", `(a|aa)+`, "a", "c", false},
	{"backtrack/nested-plus", `([a-z]+)+[0-9]`, "abcdefghij", "", false},
	{"backtrack/whitespace-token-blank", `(\s*[^\s]\s*)+`, " ", "", false},
	{"reject/general-first-rune", `([A-Za-z0-9]|[\+\?/\-:\(\)\., ]){1,2048}`, "~" + stressASCIIText, "", false},
	{"reject/general-over-length", `([A-Za-z0-9]|[\+\?/\-:\(\)\., ]){1,20}`, stressASCIIText, "", false},
	{"reject/bitset-first-rune", `(a|b)*c`, "xab", "", false},
	{"reject/scalar-fourth-rune", `(\d{3}-){2}\d{4}`, "1234567890", "", false},
	{"reject/linear-first-rune", `[A-Z]{2}[0-9]{2}[A-Za-z0-9]{1,2048}`, "1" + stressASCIIText, "", false},
}

var stressLengths = []int{32, 1024}

func TestStressCasesMatchAsDeclared(t *testing.T) {
	for _, c := range stressCases {
		pattern := mustCompile(t, c.pattern)
		for _, runes := range stressLengths {
			for name, match := range matchers() {
				matched, _, err := match(pattern, c.input(runes), unlimitedWork, nil)
				if err != nil {
					t.Fatalf("%s/%d %s: %v", c.name, runes, name, err)
				}
				if matched != c.match {
					t.Fatalf("%s/%d %s: matched = %v, want %v", c.name, runes, name, matched, c.match)
				}
			}
		}
	}
}

// BenchmarkPatternStress reports ns/rune and work/rune per case so cost per
// character compares across lengths and against other validators.
func BenchmarkPatternStress(b *testing.B) {
	for _, c := range stressCases {
		pattern := benchmarkPattern(b, c.pattern)
		for _, runes := range stressLengths {
			input := []byte(c.input(runes))
			var scratch Scratch
			b.Run(fmt.Sprintf("%s/%d", c.name, runes), func(b *testing.B) {
				b.ReportAllocs()
				var work uint64
				for b.Loop() {
					matched, used, err := pattern.MatchBytesWithScratch(input, unlimitedWork, &scratch)
					if err != nil {
						b.Fatal(err)
					}
					if matched != c.match {
						b.Fatalf("matched = %v, want %v", matched, c.match)
					}
					work = used
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(runes), "ns/rune")
				b.ReportMetric(float64(work)/float64(runes), "work/rune")
			})
		}
	}
}
