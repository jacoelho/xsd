package xsdregex

import (
	"errors"
	"strings"
	"testing"
)

func TestNodeAdmissionConstructorBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		input   string
		nodes   uint64
		oneNode bool
	}{
		{name: "empty", source: "", input: "", nodes: 1, oneNode: true},
		{name: "literal", source: "a", input: "a", nodes: 1, oneNode: true},
		{name: "concat", source: "ab", input: "ab", nodes: 3},
		{name: "alt", source: "a|b", input: "a", nodes: 3},
		{name: "repeat", source: "a*", input: "aaa", nodes: 2},
		{name: "flattened concat", source: "a(bc)", input: "abc", nodes: 5},
		{name: "flattened alt", source: "a|(b|c)", input: "b", nodes: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, maxNodes := range []uint64{test.nodes, test.nodes + 1} {
				pattern, err := compileWithLimits(test.source, compileLimits{nodes: maxNodes}.withDefaults())
				if err != nil {
					t.Fatalf("Compile(%q, MaxNodes=%d) error = %v, want nil", test.source, maxNodes, err)
				}
				matched, _, err := pattern.MatchStringWithScratch(test.input, unlimitedWork, nil)
				if err != nil || !matched {
					t.Fatalf("MatchString(%q) = %v, %v; want true, nil", test.input, matched, err)
				}
			}
			if test.oneNode {
				return
			}
			pattern, err := compileWithLimits(test.source, compileLimits{nodes: test.nodes - 1}.withDefaults())
			if pattern != nil || !IsLimit(err) {
				t.Fatalf("Compile(%q, MaxNodes=%d) = (%v, %v), want nil and limit", test.source, test.nodes-1, pattern, err)
			}
		})
	}
}

func TestNodeAdmissionErrorPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		options compileLimits
		kind    ErrorKind
		what    string
		offset  int
	}{
		{
			name:   "node exhaustion precedes later syntax",
			source: "ab[",
			options: compileLimits{
				nodes: 1,
			},
			kind:   ErrorLimit,
			what:   "compiled node count exceeds limit",
			offset: 2,
		},
		{
			name:   "syntax precedes later node exhaustion",
			source: "a[",
			options: compileLimits{
				nodes: 2,
			},
			kind:   ErrorSyntax,
			what:   "unclosed character class",
			offset: 2,
		},
		{
			name:   "range precedes node admission",
			source: `a\d`,
			options: compileLimits{
				nodes:          1,
				retainedRanges: 1,
			},
			kind:   ErrorLimit,
			what:   "pattern retained character-range count exceeds limit",
			offset: 3,
		},
		{
			name:   "depth precedes node admission",
			source: "a((b))",
			options: compileLimits{
				nodes: 1,
				depth: 1,
			},
			kind:   ErrorLimit,
			what:   "group nesting exceeds limit",
			offset: 3,
		},
		{
			name:   "byte precedes parsing",
			source: "ab",
			options: compileLimits{
				nodes:        1,
				patternBytes: 1,
			},
			kind:   ErrorLimit,
			what:   "pattern exceeds byte limit",
			offset: -1,
		},
		{
			name:   "UTF-8 precedes parsing",
			source: string([]byte{'a', 0xff}),
			options: compileLimits{
				nodes: 1,
			},
			kind:   ErrorSyntax,
			what:   "pattern is not valid UTF-8",
			offset: -1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := compileWithLimits(test.source, test.options.withDefaults())
			if pattern != nil {
				t.Fatalf("Compile(%q) returned a pattern with error %v", test.source, err)
			}
			var regexErr *Error
			if !errors.As(err, &regexErr) {
				t.Fatalf("Compile(%q) error = %v, want *Error", test.source, err)
			}
			if regexErr.Kind != test.kind || regexErr.What != test.what || regexErr.Offset != test.offset {
				t.Fatalf("Compile(%q) error = (%d, %q, %d), want (%d, %q, %d)", test.source, regexErr.Kind, regexErr.What, regexErr.Offset, test.kind, test.what, test.offset)
			}
		})
	}
}

func TestNodeAdmissionStopsParserAllocation(t *testing.T) {
	options := compileLimits{nodes: 8}.withDefaults()
	prefix := strings.Repeat("a", 8)
	shortSource := []rune(prefix + "a")
	longSource := []rune(prefix + "a" + strings.Repeat("a", 65_536))

	parserAllocs := func(source []rune) float64 {
		var (
			root *node
			err  error
		)
		allocs := testing.AllocsPerRun(3, func() {
			p := parser{source: source, limits: options}
			root, err = p.parse()
		})
		if root != nil || !IsLimit(err) {
			t.Fatalf("parse() = (%v, %v), want nil and limit", root, err)
		}
		return allocs
	}

	shortAllocs := parserAllocs(shortSource)
	longAllocs := parserAllocs(longSource)
	if longAllocs > shortAllocs+8 {
		t.Fatalf("long rejected suffix allocations = %.0f, short = %.0f; want plateau within 8 allocations", longAllocs, shortAllocs)
	}
}

// withDefaults fills unset limits from the fixed bounds so a test can tighten
// one bound without zeroing the others.
func (l compileLimits) withDefaults() compileLimits {
	f := fixedCompileLimits
	if l.patternBytes == 0 {
		l.patternBytes = f.patternBytes
	}
	if l.nodes == 0 {
		l.nodes = f.nodes
	}
	if l.states == 0 {
		l.states = f.states
	}
	if l.retainedRanges == 0 {
		l.retainedRanges = f.retainedRanges
	}
	if l.depth == 0 {
		l.depth = f.depth
	}
	return l
}
