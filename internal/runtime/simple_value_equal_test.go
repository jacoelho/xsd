package runtime

import "testing"

func TestEqualStringPatternGroupsComparesFacetProjection(t *testing.T) {
	t.Parallel()

	a := []StringPatternGroup{{Patterns: []StringPattern{equalTestPattern("ok")}}}
	b := []StringPatternGroup{{Patterns: []StringPattern{equalTestPattern("ok")}}}
	if !EqualStringPatternGroups(a, b) {
		t.Fatal("EqualStringPatternGroups() rejected equivalent pattern projections")
	}

	b[0].Patterns[0] = equalTestPattern("other")
	if EqualStringPatternGroups(a, b) {
		t.Fatal("EqualStringPatternGroups() accepted mismatched pattern projections")
	}
}

func equalTestPattern(source string) StringPattern {
	return NewFastStringPattern(source, CompileSimpleStringPattern(source))
}
