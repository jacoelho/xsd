package xsdregex

import (
	"errors"
	"testing"
)

func TestBitsetMatchWorkAccounting(t *testing.T) {
	// One 'a' transition costs decoding (1), its previous state (1), and its
	// accepting closure (1); final costs 1. An epsilon split supplies optional
	// starts and looping closures for the state-limit boundary cases.
	for _, test := range []struct {
		name, input, failure string
		start, closure       uint64
		options              MatchOptions
		matched              bool
	}{
		{"accept-at", "a", "", 1, 2, MatchOptions{MaxWork: 4}, true},
		{"accept-final-below", "a", matchWorkLimitError, 1, 2, MatchOptions{MaxWork: 3}, false},
		{"transition-below", "a", matchWorkLimitError, 1, 2, MatchOptions{MaxWork: 2}, false},
		{"dead-at", "xy", "", 1, 2, MatchOptions{MaxWork: 3}, false},
		{"dead-below", "xy", matchWorkLimitError, 1, 2, MatchOptions{MaxWork: 2}, false},
		{"empty-final-at", "", "", 3, 2, MatchOptions{MaxWork: 2}, true},
		{"empty-final-below", "", matchWorkLimitError, 3, 2, MatchOptions{MaxWork: 1}, false},
		{"initial-states-first", "", matchStateLimitError, 3, 2, MatchOptions{MaxStates: 1, MaxWork: 1}, false},
		{"next-states-first", "a", matchStateLimitError, 1, 3, MatchOptions{MaxStates: 1, MaxWork: 1}, false},
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
					matched, err = pattern.MatchBytesWithScratch([]byte(test.input), test.options, &scratch)
				} else {
					matched, err = pattern.MatchStringWithScratch(test.input, test.options, &scratch)
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
	// 1 + 3 previous states + 2 separate closure visits + 1 final state = 7.
	pattern, err := Compile(`(a?|a?)`, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range []uint64{6, 7} {
		for _, raw := range []bool{false, true} {
			var matched bool
			if raw {
				matched, err = pattern.MatchBytesWithScratch([]byte("a"), MatchOptions{MaxWork: work}, nil)
			} else {
				matched, err = pattern.MatchStringWithScratch("a", MatchOptions{MaxWork: work}, nil)
			}
			if work == 6 {
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
		pattern, err := Compile(test.source, CompileOptions{})
		if err != nil {
			t.Fatal(err)
		}
		var reused Scratch
		for _, scratch := range []*Scratch{nil, &reused, &reused} {
			matched, err := pattern.MatchStringWithScratch(test.input, MatchOptions{}, scratch)
			if err != nil || matched != test.matched {
				t.Fatalf("%q %q string: %v, %v", test.source, test.input, matched, err)
			}
			matched, err = pattern.MatchBytesWithScratch([]byte(test.input), MatchOptions{}, scratch)
			if err != nil || matched != test.matched {
				t.Fatalf("%q %q bytes: %v, %v", test.source, test.input, matched, err)
			}
		}
	}
}

func TestPatternConcurrentMatchesUseIndependentScratch(t *testing.T) {
	t.Parallel()
	pattern, err := Compile(`(a|aa)*b`, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for range 8 {
		t.Run("match", func(t *testing.T) {
			t.Parallel()
			var scratch Scratch
			for range 20 {
				matched, err := pattern.MatchStringWithScratch("aab", MatchOptions{MaxWork: 27, MaxStates: 4}, &scratch)
				if err != nil || !matched {
					t.Fatalf("independent string match = %v, %v", matched, err)
				}
				matched, err = pattern.MatchBytesWithScratch([]byte("aab"), MatchOptions{MaxWork: 27, MaxStates: 4}, &scratch)
				if err != nil || !matched {
					t.Fatalf("independent bytes match = %v, %v", matched, err)
				}
			}
		})
	}
}
