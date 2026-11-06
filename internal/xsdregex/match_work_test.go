package xsdregex

import (
	"math"
	"strings"
	"testing"
)

// The unit definition is documented on MatchOptions: one unit per input byte,
// plus NFA transition, closure, and linear row work. These tests state the
// observable contract of that definition; they do not pin exact NFA costs
// except where a derivation is given.

type matcherFunc func(*Pattern, string, MatchOptions, *Scratch) (bool, uint64, error)

func matchers() map[string]matcherFunc {
	return map[string]matcherFunc{
		"string": func(p *Pattern, input string, options MatchOptions, scratch *Scratch) (bool, uint64, error) {
			return p.MatchStringWithScratch(input, options, scratch)
		},
		"bytes": func(p *Pattern, input string, options MatchOptions, scratch *Scratch) (bool, uint64, error) {
			return p.MatchBytesWithScratch([]byte(input), options, scratch)
		},
	}
}

func TestZeroMaxWorkAdmitsNothing(t *testing.T) {
	t.Parallel()
	for _, source := range []string{`abc`, `[a-z]*`, `(a|aa)*b`, `a`, ``} {
		pattern := mustCompile(t, source)
		for _, input := range []string{"", "abc"} {
			for name, match := range matchers() {
				if _, _, err := match(pattern, input, MatchOptions{}, nil); !IsLimit(err) {
					t.Errorf("%s %q on %q: error = %v, want limit", name, source, input, err)
				}
			}
		}
	}
}

func TestMatchWorkIsExactBudget(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ source, input string }{
		{`abc`, "abc"}, {`[a-z]+`, "hello"}, {`[A-Z]{2}\d{4}`, "AB1234"},
		{`(a|aa)*b`, "aab"}, {`(a|b)*c`, "abababc"}, {`[a-z]+`, strings.Repeat("é", 300)},
	} {
		pattern := mustCompile(t, test.source)
		for name, match := range matchers() {
			for _, scratch := range []*Scratch{nil, new(Scratch)} {
				matched, work, err := match(pattern, test.input, unlimitedWork, scratch)
				if err != nil {
					t.Fatalf("%s %q: %v", name, test.source, err)
				}
				if work < uint64(len(test.input)) {
					t.Errorf("%s %q: work %d below input bytes %d", name, test.source, work, len(test.input))
				}
				got, again, err := match(pattern, test.input, MatchOptions{MaxWork: work}, scratch)
				if err != nil || got != matched || again != work {
					t.Errorf("%s %q at budget=work: %v, %d, %v; want %v, %d, nil", name, test.source, got, again, err, matched, work)
				}
				if _, _, err := match(pattern, test.input, MatchOptions{MaxWork: work - 1}, scratch); !IsLimit(err) {
					t.Errorf("%s %q at work-1: error = %v, want limit", name, test.source, err)
				}
			}
		}
		_, s, errString := matchers()["string"](pattern, test.input, unlimitedWork, nil)
		_, b, errBytes := matchers()["bytes"](pattern, test.input, unlimitedWork, nil)
		if errString != nil || errBytes != nil {
			t.Fatalf("%q: string error %v, bytes error %v", test.source, errString, errBytes)
		}
		if s != b {
			t.Errorf("%q: string work %d != bytes work %d", test.source, s, b)
		}
	}
}

func TestMatchWorkRepeatsWithReusedScratch(t *testing.T) {
	t.Parallel()
	for _, source := range []string{`(a|aa)*b`, `[a-z]+`, `(a|b)*c`} {
		pattern := mustCompile(t, source)
		input := strings.Repeat("a", 500) + "b"
		for name, match := range matchers() {
			var scratch Scratch
			_, first, err := match(pattern, input, unlimitedWork, &scratch)
			if err != nil {
				t.Fatal(err)
			}
			for range 3 {
				if _, again, err := match(pattern, input, unlimitedWork, &scratch); err != nil || again != first {
					t.Errorf("%s %q: repeated work = %d, %v; want %d", name, source, again, err, first)
				}
			}
			scratch.Reset(16)
			if _, again, err := match(pattern, input, unlimitedWork, &scratch); err != nil || again != first {
				t.Errorf("%s %q: work after Reset = %d, %v; want %d", name, source, again, err, first)
			}
		}
	}
}

func TestLongInputIsNotCappedByStateCount(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		source, input string
		want          bool
	}{
		{`[a-z]+`, strings.Repeat("a", 200_000), true},
		{`[a-z]+`, strings.Repeat("a", 200_000) + "1", false},
		{`(a|b)*c`, strings.Repeat("ab", 100_000) + "c", true},
		{`(a|b)*c`, strings.Repeat("ab", 100_000), false},
	} {
		pattern := mustCompile(t, test.source)
		for name, match := range matchers() {
			for _, scratch := range []*Scratch{nil, new(Scratch)} {
				got, _, err := match(pattern, test.input, unlimitedWork, scratch)
				if err != nil || got != test.want {
					t.Errorf("%s %q (%d bytes): %v, %v; want %v, nil", name, test.source, len(test.input), got, err, test.want)
				}
			}
		}
	}
}

// Work is bounded by 72 * (runes+1) * compiled states, the constant documented
// on MatchOptions: 66 per state per rune for the bitset runner (1 + N + N*64
// closure bits, N <= 64), plus up to 4 input bytes per rune; the general NFA
// needs at most 11. The last pattern has more than 64 states.
func TestMatchWorkIsBoundedByRunesTimesStates(t *testing.T) {
	t.Parallel()
	const k = 72
	for _, source := range []string{`(a|aa)*b`, `(a{1,20}|b)*c`, `(a|ab){1,60}x`} {
		pattern := mustCompile(t, source)
		states := uint64(len(pattern.states))
		if source == `(a|ab){1,60}x` && states <= 64 {
			t.Fatalf("%q: %d states, want more than 64", source, states)
		}
		for _, n := range []uint64{1_000, 2_000} {
			input := strings.Repeat("a", int(n)) //nolint:gosec // Test constants are small.
			var works []uint64
			for name, match := range matchers() {
				_, work, err := match(pattern, input, MatchOptions{MaxWork: math.MaxUint64}, nil)
				if err != nil {
					t.Fatal(err)
				}
				if bound := k * (n + 1) * states; work > bound {
					t.Errorf("%s %q n=%d: work %d exceeds %d", name, source, n, work, bound)
				}
				works = append(works, work)
			}
			if works[0] != works[1] {
				t.Errorf("%q n=%d: string and bytes work differ: %v", source, n, works)
			}
		}
	}
}

func TestMatchWorkIndependentOfScratchHistory(t *testing.T) {
	t.Parallel()
	pattern := mustCompile(t, `(a|ab){1,60}x`)
	if pattern.bitset || pattern.deterministic {
		t.Fatal("precondition: pattern must use the general NFA matcher")
	}
	small := mustCompile(t, `(a|ab){1,20}x`)
	if small.bitset || small.deterministic || len(small.states) >= len(pattern.states) {
		t.Fatal("precondition: smaller NFA pattern required")
	}
	input := strings.Repeat("a", 30) + "x"
	for name, match := range matchers() {
		warmed := new(Scratch)
		if _, _, err := match(pattern, input, unlimitedWork, warmed); err != nil {
			t.Fatal(err)
		}
		grown := new(Scratch)
		if _, _, err := match(small, "aax", unlimitedWork, grown); err != nil {
			t.Fatal(err)
		}
		if cap(grown.seen) >= len(pattern.states) {
			t.Fatal("precondition: scratch must need growth")
		}
		_, want, err := match(pattern, input, unlimitedWork, nil)
		if err != nil {
			t.Fatal(err)
		}
		for label, scratch := range map[string]*Scratch{"empty": {}, "warmed": warmed, "grown": grown} {
			if _, got, err := match(pattern, input, unlimitedWork, scratch); err != nil || got != want {
				t.Errorf("%s %s: work = %d, %v; want %d", name, label, got, err, want)
			}
		}
		if _, _, err := match(pattern, input, MatchOptions{MaxWork: want}, new(Scratch)); err != nil {
			t.Errorf("%s: MaxWork=work on cold scratch: %v", name, err)
		}
	}
}
