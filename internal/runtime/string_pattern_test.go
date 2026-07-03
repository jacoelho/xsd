package runtime

import (
	"regexp"
	"testing"
)

func TestCloneStringPatternGroupsDeepClonesFastPattern(t *testing.T) {
	t.Parallel()

	fast := CompileSimpleStringPattern("[A-Z]")
	if fast == nil {
		t.Fatal("CompileSimpleStringPattern returned nil")
	}
	groups := []StringPatternGroup{{
		Patterns: []StringPattern{NewFastStringPattern("[A-Z]", fast)},
	}}

	cloned := CloneStringPatternGroups(groups)
	groups[0].Patterns[0].fast.atoms = nil

	if !cloned[0].Patterns[0].MatchString("A") {
		t.Fatal("clone aliases original fast pattern")
	}
}

func TestCloneStringPatternGroupsSharesRegexpPattern(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`^[A-Z]{2}\d{2}$`)
	groups := []StringPatternGroup{{
		Patterns: []StringPattern{NewRegexpStringPattern(`[A-Z]{2}\d{2}`, `^[A-Z]{2}\d{2}$`, re)},
	}}

	cloned := CloneStringPatternGroups(groups)
	if cloned[0].Patterns[0].re != re {
		t.Fatal("regexp pattern was recompiled instead of shared")
	}
	if !cloned[0].Patterns[0].MatchString("AB12") {
		t.Fatal("shared regexp clone rejected matching value")
	}
	if cloned[0].Patterns[0].MatchString("ab12") {
		t.Fatal("shared regexp clone accepted non-matching value")
	}
}

func TestSimplePatternFastPathMatchesXSDDigitClass(t *testing.T) {
	t.Parallel()

	p := CompileSimpleStringPattern(`[A-Z]{2}\d{4}`)
	if p == nil {
		t.Fatal("CompileSimpleStringPattern() = nil")
	}
	tests := []struct {
		value string
		want  bool
	}{
		{"AB1234", true},
		{"AB" + "\u0661\u0662\u0663\u0664", true},
		{"AB" + "\uff11\uff12\uff13\uff14", true},
		{"AB123", false},
		{"ab1234", false},
		{"AB12A4", false},
	}
	for _, test := range tests {
		if got := p.MatchString(test.value); got != test.want {
			t.Fatalf("MatchString(%q) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestSimplePatternVariableFastPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		values map[string]bool
	}{
		{`[a-z]{0,3}x`, map[string]bool{"": false, "x": true, "ax": true, "abcx": true, "abcdx": false, "abc": false}},
		{`[a-z]{0,}[a-z]{0,}x`, map[string]bool{"x": true, "ax": true, "abcx": true, "abc": false}},
		{`a{1,3}a`, map[string]bool{"a": false, "aa": true, "aaa": true, "aaaa": true, "aaaaa": false}},
		{`[ab]{0,2}ab`, map[string]bool{"ab": true, "aab": true, "bab": true, "bbab": true, "aaab": true, "aaaab": false}},
		{`é{0,2}x`, map[string]bool{"x": true, "éx": true, "ééx": true, "éééx": false}},
	}
	for _, test := range tests {
		p := CompileSimpleStringPattern(test.source)
		if p == nil {
			t.Fatalf("CompileSimpleStringPattern(%q) = nil", test.source)
		}
		for value, want := range test.values {
			if got := p.MatchString(value); got != want {
				t.Fatalf("MatchString(%q against %q) = %v, want %v", value, test.source, got, want)
			}
			if got := p.MatchBytes([]byte(value)); got != want {
				t.Fatalf("MatchBytes(%q against %q) = %v, want %v", value, test.source, got, want)
			}
		}
	}
}
