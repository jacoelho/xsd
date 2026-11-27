package xsdregex

import (
	"fmt"
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

// Work is bounded by 72 * (runes+1) * tracked states, the constant documented
// on MatchOptions: 66 per state per rune for the bitset runner (1 + N + N*64
// closure bits, N <= 64), plus up to 4 input bytes per rune; the general NFA
// needs at most 11. The last pattern runs on the general runner.
func TestMatchWorkIsBoundedByRunesTimesStates(t *testing.T) {
	t.Parallel()
	const k = 72
	for _, source := range []string{`(a|aa)*b`, `(a{1,20}|b)*c`, `(a|ab){1,60}x`} {
		pattern := mustCompile(t, source)
		states := uint64(pattern.trackedSlots()) //nolint:gosec // tracked slots are bounded by the compile limit.
		if source == `(a|ab){1,60}x` && pattern.mode != modeGeneral {
			t.Fatalf("%q: runner %s, want general", source, modeName(pattern))
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

func TestCountedRepeatWorkGrowsLinearlyWithBoundUnlessMinimumIsLarge(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ source, unit string }{
		{`([A-Za-z0-9]|[\+\?/\-:\(\)\., ]){1,%d}`, "a"},
		{`([a-z]+[0-9]){1,%d}`, "a1"},
		{`([a-z]+){1,%d}`, "a"},
		{`(a|aa){1,%d}`, "a"},
		{`(a?){1,%d}`, "a"},
		{`(a?){%d}`, "a"},
		{`(a{0,2}){1,%d}`, "a"},
		{`([a-z]*[0-9]?){1,%d}`, "a"},
		{`(a|aa){3,%d}`, "a"},
	} {
		var previous uint64
		for _, bound := range []int{100, 200, 400} {
			source := fmt.Sprintf(c.source, bound)
			pattern := mustCompile(t, source)
			if pattern.mode != modeGeneral {
				t.Fatalf("%s: runner %s, want general", source, modeName(pattern))
			}
			matched, work, err := pattern.MatchStringWithScratch(strings.Repeat(c.unit, bound), unlimitedWork, nil)
			if err != nil || !matched {
				t.Fatalf("%s: matched=%v err=%v", source, matched, err)
			}
			if previous != 0 && work > 3*previous {
				t.Errorf("%s: doubling the bound and input raised work from %d to %d, want at most %d", source, previous, work, 3*previous)
			}
			previous = work
		}
	}
}

func TestMatchWorkIndependentOfScratchHistory(t *testing.T) {
	t.Parallel()
	pattern := mustCompile(t, `(a|ab){1,60}x`)
	if pattern.mode == modeBitset || pattern.mode == modeScalar {
		t.Fatal("precondition: pattern must use the general NFA matcher")
	}
	small := mustCompile(t, `(a|ab){1,20}x`)
	if small.mode == modeBitset || small.mode == modeScalar || small.trackedSlots() >= pattern.trackedSlots() {
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
		if cap(grown.seen) >= pattern.trackedSlots() {
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

// Each offset plants markers equal to one post-wrap generation of the large
// match, so a stale marker would read as a visit or as a smaller count
// already present in that generation.
func TestMatchAfterGenerationWrapIgnoresStaleMarkers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, small, large, largeInput string
		plant                          func(s *Scratch, small *Pattern, marker uint32) (planted int)
	}{
		{
			name: "visits", small: `(a|ab){1,20}x`, large: `(a|ab){1,60}x`, largeInput: strings.Repeat("a", 30) + "x",
			plant: func(s *Scratch, small *Pattern, marker uint32) int {
				stale := s.seen[:cap(s.seen)]
				for i := small.trackedSlots(); i < len(stale); i++ {
					stale[i] = marker
				}
				return len(stale) - small.trackedSlots()
			},
		},
		{
			name: "dominance", small: `(a|ab){1,20}x`, large: `(a|ab){1,20}x(b|bc){1,40}y`, largeInput: "aax" + strings.Repeat("b", 20) + "y",
			plant: func(s *Scratch, small *Pattern, marker uint32) int {
				stale := s.least[:cap(s.least)]
				for i := len(small.states); i < len(stale); i++ {
					stale[i] = leastCount{mark: marker}
				}
				return len(stale) - len(small.states)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			small, large := mustCompile(t, test.small), mustCompile(t, test.large)
			if small.counted == nil || large.counted == nil {
				t.Fatal("precondition: two counted patterns")
			}
			const smallInput = "aax"
			for name, match := range matchers() {
				counted := new(Scratch)
				if _, _, err := match(small, smallInput, unlimitedWork, counted); err != nil {
					t.Fatal(err)
				}
				closures := counted.mark
				wantMatched, wantWork, err := match(large, test.largeInput, unlimitedWork, nil)
				if err != nil {
					t.Fatal(err)
				}
				for offset := uint32(1); offset <= uint32(len(test.largeInput))+2; offset++ { //nolint:gosec // the input is short.
					scratch := new(Scratch)
					if _, _, err := match(large, test.largeInput, unlimitedWork, scratch); err != nil {
						t.Fatal(err)
					}
					if test.plant(scratch, small, closures+offset) <= 0 {
						t.Fatal("precondition: the large match leaves markers past the small pattern's")
					}
					scratch.mark = ^uint32(0)
					if _, _, err := match(small, smallInput, unlimitedWork, scratch); err != nil {
						t.Fatal(err)
					}
					if scratch.mark != closures {
						t.Fatalf("%s: precondition: mark after wrap = %d, want %d", name, scratch.mark, closures)
					}
					matched, work, err := match(large, test.largeInput, unlimitedWork, scratch)
					if err != nil || matched != wantMatched || work != wantWork {
						t.Errorf("%s offset %d: after wrap = %v, %d, %v; want %v, %d", name, offset, matched, work, err, wantMatched, wantWork)
					}
				}
			}
		})
	}
}

func TestScratchResetClearsDominanceMarkers(t *testing.T) {
	t.Parallel()
	pattern := mustCompile(t, `(a|ab){1,20}x(b|bc){1,40}y`)
	input := "aax" + strings.Repeat("b", 20) + "y"
	for name, match := range matchers() {
		wantMatched, wantWork, err := match(pattern, input, unlimitedWork, nil)
		if err != nil || !wantMatched {
			t.Fatalf("precondition: match = %v, %v", wantMatched, err)
		}
		scratch := new(Scratch)
		if _, _, err = match(pattern, input, unlimitedWork, scratch); err != nil {
			t.Fatal(err)
		}
		least := scratch.least[:cap(scratch.least)]
		for i := range least {
			least[i] = leastCount{mark: 1}
		}
		scratch.Reset(1 << 20)
		matched, work, err := match(pattern, input, unlimitedWork, scratch)
		if err != nil || matched != wantMatched || work != wantWork {
			t.Errorf("%s: after Reset = %v, %d, %v; want %v, %d", name, matched, work, err, wantMatched, wantWork)
		}
	}
}
