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
				pattern, err := Compile(test.source, CompileOptions{MaxNodes: maxNodes})
				if err != nil {
					t.Fatalf("Compile(%q, MaxNodes=%d) error = %v, want nil", test.source, maxNodes, err)
				}
				matched, err := pattern.MatchString(test.input)
				if err != nil || !matched {
					t.Fatalf("MatchString(%q) = %v, %v; want true, nil", test.input, matched, err)
				}
			}
			if test.oneNode {
				return
			}
			pattern, err := Compile(test.source, CompileOptions{MaxNodes: test.nodes - 1})
			if pattern != nil || !IsLimit(err) {
				t.Fatalf("Compile(%q, MaxNodes=%d) = (%v, %v), want nil and limit", test.source, test.nodes-1, pattern, err)
			}
		})
	}
}

func TestNodeAdmissionZeroUsesDefault(t *testing.T) {
	for _, source := range []string{"", "a", "a*"} {
		if pattern, err := Compile(source, CompileOptions{MaxNodes: 0}); err != nil || pattern == nil {
			t.Errorf("Compile(%q, MaxNodes=0) = (%v, %v), want pattern and nil error", source, pattern, err)
		}
	}
}

func TestNodeAdmissionErrorPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		options CompileOptions
		kind    ErrorKind
		what    string
		offset  int
	}{
		{
			name:   "node exhaustion precedes later syntax",
			source: "ab[",
			options: CompileOptions{
				MaxNodes: 1,
			},
			kind:   ErrorLimit,
			what:   "compiled node count exceeds limit",
			offset: 2,
		},
		{
			name:   "syntax precedes later node exhaustion",
			source: "a[",
			options: CompileOptions{
				MaxNodes: 2,
			},
			kind:   ErrorSyntax,
			what:   "unclosed character class",
			offset: 2,
		},
		{
			name:   "range precedes node admission",
			source: `a\d`,
			options: CompileOptions{
				MaxNodes:  1,
				MaxRanges: 1,
			},
			kind:   ErrorLimit,
			what:   "pattern range count exceeds limit",
			offset: 3,
		},
		{
			name:   "depth precedes node admission",
			source: "a((b))",
			options: CompileOptions{
				MaxNodes: 1,
				MaxDepth: 1,
			},
			kind:   ErrorLimit,
			what:   "group nesting exceeds limit",
			offset: 3,
		},
		{
			name:   "repeat precedes node admission",
			source: "a{2}",
			options: CompileOptions{
				MaxNodes:  1,
				MaxRepeat: 1,
			},
			kind:   ErrorLimit,
			what:   "counted quantifier exceeds repeat limit",
			offset: 3,
		},
		{
			name:   "byte precedes parsing",
			source: "ab",
			options: CompileOptions{
				MaxNodes:        1,
				MaxPatternBytes: 1,
			},
			kind:   ErrorLimit,
			what:   "pattern exceeds byte limit",
			offset: -1,
		},
		{
			name:   "UTF-8 precedes parsing",
			source: string([]byte{'a', 0xff}),
			options: CompileOptions{
				MaxNodes: 1,
			},
			kind:   ErrorSyntax,
			what:   "pattern is not valid UTF-8",
			offset: -1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := Compile(test.source, test.options)
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
	options := normalizeCompileOptions(CompileOptions{MaxNodes: 8})
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
