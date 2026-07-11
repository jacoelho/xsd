package runtime

import (
	"math"
	"testing"
)

func newTestStringPatternSteps(groups [][]StringPattern) stringPatternSteps {
	var steps stringPatternSteps
	for _, patterns := range groups {
		steps = appendStringPatternStep(steps, patterns)
	}
	return steps
}

func TestStringPatternSourcesRejectCorruptChains(t *testing.T) {
	t.Parallel()

	pattern := NewFastStringPattern(CompileSimpleStringPattern("[A-Z]"))
	self := &stringPatternStep{patterns: []StringPattern{pattern}, count: 1}
	self.parent = self
	first := &stringPatternStep{patterns: []StringPattern{pattern}, count: 1}
	second := &stringPatternStep{parent: first, patterns: []StringPattern{pattern}, count: 2}
	first.parent = second
	tests := []struct {
		name string
		tail *stringPatternStep
	}{
		{name: "self cycle", tail: self},
		{name: "multi-node cycle", tail: second},
		{name: "count mismatch", tail: &stringPatternStep{patterns: []StringPattern{pattern}, count: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			types := []SimpleType{{Facets: FacetSet{patterns: stringPatternSteps{tail: tt.tail}}}}
			if err := validateStringPatternSourcesForSimpleTypes(types); err == nil {
				t.Fatal("validateStringPatternSourcesForSimpleTypes() accepted corrupt chain")
			}
		})
	}
}

func TestStringPatternReadBuildHandlesDeepChain(t *testing.T) {
	t.Parallel()

	pattern := NewFastStringPattern(CompileSimpleStringPattern("[A-Z]"))
	var steps stringPatternSteps
	var midpoint *stringPatternStep
	for i := range 10_000 {
		steps = appendStringPatternStep(steps, []StringPattern{pattern})
		if i == 4_999 {
			midpoint = steps.tail
		}
	}
	if err := validateStringPatternSourcesForSimpleTypes([]SimpleType{{Facets: FacetSet{patterns: steps}}}); err != nil {
		t.Fatalf("validateStringPatternSourcesForSimpleTypes() error = %v", err)
	}
	types := []SimpleType{{Facets: FacetSet{patterns: steps}}}
	pool := newStringPatternReadPoolForSimpleTypes(types)
	read := pool[steps.tail]
	if read == nil || read.count != 10_000 || pool[midpoint] == nil {
		t.Fatalf("deep pattern read = %#v, midpoint = %#v", read, pool[midpoint])
	}
	if read.parent == nil {
		t.Fatal("deep pattern read omitted pooled parent")
	}
}

func TestStringPatternReadPoolPreservesSharingAndBoundaries(t *testing.T) {
	t.Parallel()

	a := NewFastStringPattern(CompileSimpleStringPattern("a"))
	b := NewFastStringPattern(CompileSimpleStringPattern("b"))
	base := appendStringPatternStep(stringPatternSteps{}, []StringPattern{a})
	left := appendStringPatternStep(base, []StringPattern{a})
	right := appendStringPatternStep(base, []StringPattern{b})
	types := []SimpleType{
		{},
		{Facets: FacetSet{patterns: base}},
		{Facets: FacetSet{patterns: left}},
		{Facets: FacetSet{patterns: left}},
		{Facets: FacetSet{patterns: right}},
	}

	pool := newStringPatternReadPoolForSimpleTypes(types)
	if pool[nil] != nil {
		t.Fatal("nil pattern source has a read")
	}
	if pool[left.tail] == nil || pool[left.tail] != pool[types[3].Facets.patterns.tail] {
		t.Fatal("shared pattern tail did not reuse one read")
	}
	if pool[left.tail].parent != pool[base.tail] || pool[right.tail].parent != pool[base.tail] {
		t.Fatal("shared pattern ancestor did not reuse one read")
	}
	for source, read := range pool {
		if cap(read.patterns) != len(read.patterns) {
			t.Fatalf("pattern step %p capacity = %d/%d", source, len(read.patterns), cap(read.patterns))
		}
	}
	pool[left.tail].patterns[0] = stringPatternRead{}
	if pool[right.tail].patterns[0].fast == nil || pool[base.tail].patterns[0].fast == nil {
		t.Fatal("pattern matcher subslices overlap")
	}
}

var stringPatternReadPoolAllocationSink map[*stringPatternStep]*stringPatternStepRead

func TestStringPatternReadPoolAllocationCountIsBounded(t *testing.T) {
	pattern := NewFastStringPattern(CompileSimpleStringPattern("a"))
	var steps stringPatternSteps
	for range 10_000 {
		steps = appendStringPatternStep(steps, []StringPattern{pattern})
	}
	types := []SimpleType{{Facets: FacetSet{patterns: steps}}}

	allocs := testing.AllocsPerRun(3, func() {
		stringPatternReadPoolAllocationSink = newStringPatternReadPoolForSimpleTypes(types)
	})
	if allocs > 128 {
		t.Fatalf("newStringPatternReadPoolForSimpleTypes() allocations = %v, want at most 128", allocs)
	}
}

func TestAddStringPatternReadCountRejectsOverflow(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("addStringPatternReadCount() accepted overflowing count")
		}
	}()
	addStringPatternReadCount(math.MaxInt, 1)
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
