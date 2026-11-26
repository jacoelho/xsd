package xsdregex

import (
	"strings"
	"testing"
)

func modeName(p *Pattern) string {
	switch p.mode {
	case modeLinear:
		return "linear"
	case modeScalar:
		return "scalar"
	case modeBitset:
		return "bitset"
	case modeGeneral:
		return "general"
	}
	return "unknown"
}

func TestMatchRunnerModes(t *testing.T) {
	t.Parallel()

	type input struct {
		text string
		want bool
		work uint64
	}
	tests := []struct {
		name   string
		source string
		mode   string
		inputs []input
	}{
		{"linear", `abc`, "linear", []input{
			{"abc", true, 3}, {"abd", false, 3}, {"", false, 0},
		}},
		{"bitset", `(a|b)*c`, "bitset", []input{
			{"c", true, 7}, {"abac", true, 31}, {"abab", false, 35}, {"", false, 3},
		}},
		{"general large", `(a|b){33000}`, "general", []input{
			{strings.Repeat("ab", 16500), true, 396004}, {strings.Repeat("a", 32999), false, 395998}, {"", false, 10},
		}},
		{"scalar", `(\d{3}-){2}\d{4}`, "scalar", []input{
			{"123-456-7890", true, 37}, {"123-456-789", false, 34}, {"123-45x-7890", false, 32}, {"", false, 1},
		}},
		{"general over 64 states", `(a|b)*a(a|b){70}`, "general", []input{
			{"a" + strings.Repeat("b", 70), true, 1997}, {strings.Repeat("b", 71), false, 1296},
			{"ba" + strings.Repeat("b", 70), true, 2015}, {"", false, 18},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := Compile(tt.source)
			if err != nil {
				t.Fatal(err)
			}
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
