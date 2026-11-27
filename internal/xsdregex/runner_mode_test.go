package xsdregex

import (
	"strings"
	"testing"
)

func modeName(p *Pattern) string {
	switch p.mode {
	case modeLinear:
		return "linear"
	case modeBitset:
		return "bitset"
	case modeGeneral:
		return "general"
	case modeDFA:
		return "dfa"
	}
	return "unknown"
}

func TestRunnerSelection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ source, runner string }{
		{`abc`, "linear"},
		{`[A-Z]{2}\d{4}`, "dfa"},
		{`(a|b)*c`, "dfa"},
		{`(a|ab){1,60}x`, "dfa"},
		{`[a-z]{1,5000}`, "linear"},
		{`[a-z]{60000}[a-z]{60000}`, "linear"},
		{`x{1,100000}`, "linear"},
		{`(a|b)*a(a|b){12}`, "bitset"},
		{`(a|b)*a(a|b){70}`, "general"},
		{`(a|b){33000}`, "general"},
		{`(a|aa){1,19000}`, "general with counters"},
	} {
		pattern := mustCompile(t, test.source)
		runner := modeName(pattern)
		if pattern.counted != nil {
			runner += " with counters"
		}
		if runner != test.runner {
			t.Errorf("%q: runner %s, want %s", test.source, runner, test.runner)
		}
	}
}

func TestMatchRunnerModes(t *testing.T) {
	t.Parallel()

	type input struct {
		text string
		want bool
		work uint64
	}
	tests := []struct {
		name    string
		source  string
		mode    string
		compile func(testing.TB, string) *Pattern
		inputs  []input
	}{
		{"literal", `abc`, "linear", mustCompile, []input{
			{"abc", true, 3}, {"abd", false, 3}, {"", false, 0},
		}},
		{"table for a star", `(a|b)*c`, "dfa", mustCompile, []input{
			{"c", true, 2}, {"abac", true, 8}, {"abab", false, 8}, {"", false, 0}, {"abxab", false, 8},
		}},
		{"table for fixed counts", `(\d{3}-){2}\d{4}`, "dfa", mustCompile, []input{
			{"123-456-7890", true, 24}, {"123-456-789", false, 22}, {"123-45x-7890", false, 19}, {"", false, 0},
			{"١٢٣-456-7890", true, 27},
		}},
		{"run lengths", `[a-z]{1,5000}`, "linear", mustCompile, []input{
			{"abc", true, 20}, {"abC", false, 20}, {"", false, 5},
		}},
		{"bitset", `(a|b)*c`, "bitset", mustCompileWithoutDFA, []input{
			{"c", true, 7}, {"abac", true, 31}, {"abab", false, 35}, {"", false, 3},
		}},
		{"general large", `(a|b){33000}`, "general", mustCompile, []input{
			{strings.Repeat("ab", 16500), true, 396004}, {strings.Repeat("a", 32999), false, 395998}, {"", false, 10},
		}},
		{"general over 64 states", `(a|b)*a(a|b){70}`, "general", mustCompile, []input{
			{"a" + strings.Repeat("b", 70), true, 1997}, {strings.Repeat("b", 71), false, 1296},
			{"ba" + strings.Repeat("b", 70), true, 2015}, {"", false, 18},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := tt.compile(t, tt.source)
			if got := modeName(p); got != tt.mode {
				t.Fatalf("mode = %s, want %s", got, tt.mode)
			}
			var reused Scratch
			for _, in := range tt.inputs {
				options := MatchOptions{MaxWork: 1 << 40}
				for _, scratch := range []*Scratch{nil, &reused} {
					got, work, err := matchString(p, in.text, options, scratch)
					if err != nil || got != in.want || work != in.work {
						t.Errorf("string %.20q (len %d) scratch=%v = %v, %d, %v; want %v, %d", in.text, len(in.text), scratch != nil, got, work, err, in.want, in.work)
					}
					got, work, err = matchBytes(p, []byte(in.text), options, scratch)
					if err != nil || got != in.want || work != in.work {
						t.Errorf("bytes %.20q (len %d) scratch=%v = %v, %d, %v; want %v, %d", in.text, len(in.text), scratch != nil, got, work, err, in.want, in.work)
					}
				}
			}
		})
	}
}
