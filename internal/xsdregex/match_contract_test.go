package xsdregex

import (
	"errors"
	"math"
	"testing"
)

func TestBitsetMatchWorkAccounting(t *testing.T) {
	// Unit definition: one per input byte, then per rune one for the step, one
	// per active state, and one per closure state reached; the final accept
	// check costs one per active state. For "a": 1 + (1+1+1) + 1 = 5. For
	// "xy" (no match): 2 + (1+1) + (1+0) + 0 = 5.
	for _, test := range []struct {
		name, input, failure string
		start, closure       uint64
		options              MatchOptions
		matched              bool
	}{
		{"accept-at", "a", "", 1, 2, MatchOptions{MaxWork: 5}, true},
		{"accept-final-below", "a", matchWorkLimitError, 1, 2, MatchOptions{MaxWork: 4}, false},
		{"transition-below", "a", matchWorkLimitError, 1, 2, MatchOptions{MaxWork: 3}, false},
		{"dead-at", "xy", "", 1, 2, MatchOptions{MaxWork: 5}, false},
		{"dead-below", "xy", matchWorkLimitError, 1, 2, MatchOptions{MaxWork: 4}, false},
		{"empty-final-at", "", "", 3, 2, MatchOptions{MaxWork: 2}, true},
		{"empty-final-below", "", matchWorkLimitError, 3, 2, MatchOptions{MaxWork: 1}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			pattern := &Pattern{
				states: []instruction{
					{kind: instructionSet, set: singletonSet('a'), out: 1},
					{kind: instructionAccept},
					{kind: instructionSplit, out: 0, out1: 1},
				},
				bitset: true, startBits: test.start, acceptBits: 2, closureBits: []uint64{test.closure, 0, 0},
			}
			if test.start == 3 {
				pattern.start = 2
			}
			if test.closure == 3 {
				pattern.states[0].out = 2
			}
			var scratch Scratch
			for _, raw := range []bool{false, true} {
				var matched bool
				var err error
				if raw {
					matched, _, err = pattern.MatchBytesWithScratch([]byte(test.input), test.options, &scratch)
				} else {
					matched, _, err = pattern.MatchStringWithScratch(test.input, test.options, &scratch)
				}
				if test.failure == "" {
					if err != nil || matched != test.matched {
						t.Fatalf("bytes=%v: match = %v, %v", raw, matched, err)
					}
				} else {
					assertMatchLimitError(t, err, test.failure)
				}
			}
		})
	}
}

func assertMatchLimitError(t *testing.T, err error, what string) {
	t.Helper()
	failure, ok := errors.AsType[*Error](err)
	if !ok || *failure != (Error{Kind: ErrorLimit, Offset: -1, What: what}) {
		t.Fatalf("error = %v, want limit %q at offset -1", err, what)
	}
}

func TestBitsetMatchChargesOverlappingClosures(t *testing.T) {
	// Both optional 'a' states reach the same accept state. After 'a', charge
	// 1 byte + 1 step + 3 previous states + 2 separate closure visits + 1 final
	// state = 8.
	pattern, err := Compile(`(a?|a?)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range []uint64{7, 8} {
		for _, raw := range []bool{false, true} {
			var matched bool
			if raw {
				matched, _, err = pattern.MatchBytesWithScratch([]byte("a"), MatchOptions{MaxWork: work}, nil)
			} else {
				matched, _, err = pattern.MatchStringWithScratch("a", MatchOptions{MaxWork: work}, nil)
			}
			if work == 7 {
				assertMatchLimitError(t, err, matchWorkLimitError)
			} else if err != nil || !matched {
				t.Fatalf("match = %v, %v", matched, err)
			}
		}
	}
}

func TestPatternLanguageAndScratchReuse(t *testing.T) {
	for _, test := range []struct {
		source, input string
		matched       bool
	}{
		{`a|ab`, "ab", true}, {`a|ab`, "aa", false},
		{`([a-z-[aeiou]]|[0-9])+`, "b3z", true}, {`([a-z-[aeiou]]|[0-9])+`, "be", false},
		{`([^a]|ab)+`, "éab", true}, {`([^a]|ab)+`, "a", false},
		{`(\s*[^\s]\s*)+`, " a\tβ x ", true}, {`(\s*[^\s]\s*)+`, " \t", false},
		{`(é|éa)*z`, "ééaz", true}, {`(é|éa)*z`, "ééax", false},
	} {
		pattern, err := Compile(test.source)
		if err != nil {
			t.Fatal(err)
		}
		var reused Scratch
		for _, scratch := range []*Scratch{nil, &reused, &reused} {
			matched, _, err := pattern.MatchStringWithScratch(test.input, unlimitedWork, scratch)
			if err != nil || matched != test.matched {
				t.Fatalf("%q %q string: %v, %v", test.source, test.input, matched, err)
			}
			matched, _, err = pattern.MatchBytesWithScratch([]byte(test.input), unlimitedWork, scratch)
			if err != nil || matched != test.matched {
				t.Fatalf("%q %q bytes: %v, %v", test.source, test.input, matched, err)
			}
		}
	}
}

func TestPatternConcurrentMatchesUseIndependentScratch(t *testing.T) {
	t.Parallel()
	pattern, err := Compile(`(a|aa)*b`)
	if err != nil {
		t.Fatal(err)
	}
	for range 8 {
		t.Run("match", func(t *testing.T) {
			t.Parallel()
			var scratch Scratch
			for range 20 {
				matched, _, err := pattern.MatchStringWithScratch("aab", unlimitedWork, &scratch)
				if err != nil || !matched {
					t.Fatalf("independent string match = %v, %v", matched, err)
				}
				matched, _, err = pattern.MatchBytesWithScratch([]byte("aab"), unlimitedWork, &scratch)
				if err != nil || !matched {
					t.Fatalf("independent bytes match = %v, %v", matched, err)
				}
			}
		})
	}
}

var unlimitedWork = MatchOptions{MaxWork: math.MaxUint64}
