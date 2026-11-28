package xsdregex

import (
	"strings"
	"testing"
)

type scratchCase struct {
	name    string
	source  string
	input   string
	runner  string
	counted bool
}

var scratchCases = []scratchCase{
	{name: "general", source: `(a|b)*a(a|b){70}`, input: strings.Repeat("ab", 150), runner: "general"},
	{name: "counted", source: `(a|ab){1,60}x`, input: strings.Repeat("a", 30) + "x", runner: "general", counted: true},
	{name: "run-length", source: `[a-c]{1,9000}`, input: strings.Repeat("abc", 100), runner: "linear"},
}

func (c scratchCase) compile(tb testing.TB) *Pattern {
	tb.Helper()
	pattern := mustCompileWithoutDFA(tb, c.source)
	if modeName(pattern) != c.runner || (pattern.counted != nil) != c.counted {
		tb.Fatalf("precondition: %s runs on %s (counted %t), want %s (counted %t)", c.source, modeName(pattern), pattern.counted != nil, c.runner, c.counted)
	}
	return pattern
}

func (c scratchCase) entryPoints(pattern *Pattern, scratch *Scratch) map[string]func() error {
	raw := []byte(c.input)
	return map[string]func() error{
		"string": func() error {
			_, _, err := pattern.MatchStringWithScratch(c.input, unlimitedWork, scratch)
			return err
		},
		"bytes": func() error {
			_, _, err := pattern.MatchBytesWithScratch(raw, unlimitedWork, scratch)
			return err
		},
	}
}

func TestReusedScratchMatchesWithoutAllocating(t *testing.T) {
	for _, c := range scratchCases {
		var scratch Scratch
		for entry, match := range c.entryPoints(c.compile(t), &scratch) {
			var err error
			allocations := testing.AllocsPerRun(20, func() { err = match() })
			if err != nil || allocations != 0 {
				t.Errorf("%s %s: %v allocations per match, %v; want 0", c.name, entry, allocations, err)
			}
		}
	}
}

func TestScratchResetReleasesOnlyStorageAboveItsBound(t *testing.T) {
	for _, c := range scratchCases {
		var scratch Scratch
		for entry, match := range c.entryPoints(c.compile(t), &scratch) {
			var err error
			unused := testing.AllocsPerRun(20, func() {
				scratch = Scratch{}
				err = match()
			})
			if err != nil || unused == 0 {
				t.Fatalf("%s %s: precondition: %v allocations on an unused Scratch, %v; want some", c.name, entry, unused, err)
			}
			kept := testing.AllocsPerRun(20, func() {
				scratch.Reset(1 << 20)
				err = match()
			})
			if err != nil || kept != 0 {
				t.Errorf("%s %s: %v allocations after Reset above the storage in use, %v; want 0", c.name, entry, kept, err)
			}
			for _, bound := range []int{0, -1} {
				released := testing.AllocsPerRun(20, func() {
					scratch.Reset(bound)
					err = match()
				})
				if err != nil || released != unused {
					t.Errorf("%s %s: %v allocations after Reset(%d), %v; want the %v of an unused Scratch", c.name, entry, released, bound, err, unused)
				}
			}
		}
	}
}

func TestMatchAfterExhaustedWorkEqualsMatchWithoutScratch(t *testing.T) {
	t.Parallel()
	for _, c := range scratchCases {
		pattern := c.compile(t)
		for entry, match := range matchers() {
			wantMatched, wantWork, err := match(pattern, c.input, unlimitedWork, nil)
			if err != nil {
				t.Fatal(err)
			}
			scratch := new(Scratch)
			for limit := uint64(1); limit < wantWork; limit += wantWork/97 + 1 {
				if _, _, err := match(pattern, c.input, MatchOptions{MaxWork: limit}, scratch); !IsLimit(err) {
					t.Fatalf("%s %s: precondition: MaxWork %d of %d = %v, want a limit error", c.name, entry, limit, wantWork, err)
				}
				matched, work, err := match(pattern, c.input, unlimitedWork, scratch)
				if err != nil || matched != wantMatched || work != wantWork {
					t.Fatalf("%s %s: after a match stopped at work %d = %v, %d, %v; want %v, %d", c.name, entry, limit, matched, work, err, wantMatched, wantWork)
				}
			}
		}
	}
}
