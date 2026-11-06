package xsdregex

import (
	"strings"
	"testing"
)

func TestFixedPatternLanguageAndReuse(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		source, input string
		matched       bool
	}{
		{`[A-Z]{2}\d{4}`, "AB1234", true},
		{`[A-Z]{2}\d{4}`, "AB١٢٣٤", true},
		{`[A-Z]{2}\d{4}`, "AB123", false},
		{`[A-Z]{2}\d{4}`, "AB12345", false},
		{`[A-Z]{2}\d{4}`, "aB1234", false},
		{`[A-Z]{2}\d{4}`, "AB123x", false},
		{`[A-Z]{2}\d{4}`, "AB１２３４", true},
		{`[A-Z]{2}\d{4}`, "AB¹²³⁴", false},
		{`[a-z-[aeiou]]{2}`, "bz", true},
		{`[a-z-[aeiou]]{2}`, "be", false},
		{`[^a]{2}`, "éβ", true},
		{`[^a]{2}`, "éa", false},
		{`[A-Z]{0}\d{2}`, "١٢", true},
		{`[A-Z]{0}\d{2}`, "A12", false},
		{``, "", true},
		{``, "x", false},
		{`a{1,2}`, "a", true},
		{`a{1,2}`, "aa", true},
		{`a{1,2}`, "aaa", false},
	} {
		pattern := mustCompile(t, test.source)
		var reused Scratch
		for _, scratch := range []*Scratch{nil, &reused, &reused} {
			for representation, match := range fixedContractMatchers(pattern) {
				matched, err := match(test.input, unlimitedWork, scratch)
				if err != nil || matched != test.matched {
					t.Fatalf("pattern=%q input=%q representation=%d: %v, %v; want %v", test.source, test.input, representation, matched, err, test.matched)
				}
			}
		}
	}
}

func TestFixedPatternResourceBoundaries(t *testing.T) {
	t.Parallel()
	pattern := mustCompile(t, `[A-Z]{2}\d{4}`)
	for _, test := range []struct {
		input, failure string
		work           uint64
		matched        bool
	}{
		{"AB1234", "", 64, true},
		{"AB١٢٣٤", "", 68, true},
		{"aB1234", "", 64, false},
		{"AB1234", matchWorkLimitError, 63, false},
		{"aB1234", matchWorkLimitError, 63, false},
		{"AB123", "", 55, false},
		{"AB123", matchWorkLimitError, 54, false},
		{"AB12345", "", 73, false},
		{"AB12345", matchWorkLimitError, 72, false},
		{"", "", 10, false},
		{"", matchWorkLimitError, 9, false},
	} {
		var reused Scratch
		for _, scratch := range []*Scratch{nil, &reused, &reused} {
			for representation, match := range fixedContractMatchers(pattern) {
				matched, err := match(test.input, MatchOptions{MaxWork: test.work}, scratch)
				if test.failure != "" {
					assertMatchLimitError(t, err, test.failure)
				} else if err != nil || matched != test.matched {
					t.Fatalf("input=%q work=%d representation=%d: %v, %v; want %v", test.input, test.work, representation, matched, err, test.matched)
				}
			}
		}
	}
}

func TestFixedPatternLongScratchAndLiteralBudget(t *testing.T) {
	t.Parallel()
	pattern := mustCompile(t, `[A-Z]{129}`)
	input := strings.Repeat("A", 129)
	var reused Scratch
	for representation, match := range fixedContractMatchers(pattern) {
		for _, work := range []uint64{650, 649, 650} {
			matched, err := match(input, MatchOptions{MaxWork: work}, &reused)
			if work == 649 {
				assertMatchLimitError(t, err, matchWorkLimitError)
			} else if err != nil || !matched {
				t.Fatalf("long input work=%d representation=%d: %v, %v", work, representation, matched, err)
			}
		}
	}
	literal := mustCompile(t, `é`)
	for representation, match := range fixedContractMatchers(literal) {
		matched, err := match("é", MatchOptions{MaxWork: 2}, &reused)
		if err != nil || !matched {
			t.Fatalf("literal representation=%d: %v, %v", representation, matched, err)
		}
		_, err = match("é", MatchOptions{MaxWork: 1}, &reused)
		assertMatchLimitError(t, err, matchWorkLimitError)
	}
}

func fixedContractMatchers(pattern *Pattern) [2]func(string, MatchOptions, *Scratch) (bool, error) {
	return [2]func(string, MatchOptions, *Scratch) (bool, error){
		func(input string, options MatchOptions, scratch *Scratch) (bool, error) {
			matched, _, err := pattern.MatchStringWithScratch(input, options, scratch)
			return matched, err
		},
		func(input string, options MatchOptions, scratch *Scratch) (bool, error) {
			matched, _, err := pattern.MatchBytesWithScratch([]byte(input), options, scratch)
			return matched, err
		},
	}
}
