package xsdregex

import (
	"strconv"
	"strings"
	"testing"
)

func BenchmarkCompileRepeatedCategoryTerms(b *testing.B) {
	for _, count := range []int{1, 16, 128} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			source := `[` + strings.Repeat(`\p{L}`, count) + `]`
			b.ReportAllocs()
			for b.Loop() {
				pattern, err := Compile(source)
				if err != nil {
					b.Fatal(err)
				}
				matched, _, err := pattern.MatchStringWithScratch("A", unlimitedWork, nil)
				if err != nil || !matched {
					b.Fatalf("MatchString(A) = %v, %v", matched, err)
				}
			}
		})
	}
}

func BenchmarkCompileBitsetWhitespace(b *testing.B) {
	const source = `(\s*[^\s]\s*)+`
	pattern := benchmarkPattern(b, source)
	if !pattern.bitset || pattern.deterministic || pattern.linear != nil {
		b.Fatal("benchmark requires the general bitset NFA")
	}
	if matched, _, err := pattern.MatchStringWithScratch(" a ", unlimitedWork, nil); err != nil || !matched {
		b.Fatalf("fixture match = %v, %v", matched, err)
	}
	b.ReportAllocs()
	for b.Loop() {
		var err error
		pattern, err = Compile(source)
		if err != nil || pattern == nil {
			b.Fatalf("compile = %v, %v", pattern, err)
		}
	}
	if matched, _, err := pattern.MatchStringWithScratch(" a ", unlimitedWork, nil); err != nil || !matched {
		b.Fatalf("compiled match = %v, %v", matched, err)
	}
}

func BenchmarkParseNodeAdmissionPrebuilt(b *testing.B) {
	options := compileLimits{nodes: 8}.withDefaults()
	for _, test := range []struct {
		name string
		size int
	}{
		{name: "64", size: 64},
		{name: "65536", size: 65_536},
		{name: "1MiB", size: 1 << 20},
	} {
		source := []rune(strings.Repeat("a", test.size))
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				p := parser{source: source, limits: options}
				root, err := p.parse()
				if root != nil || !IsLimit(err) {
					b.Fatalf("parse() = (%v, %v), want nil and limit", root, err)
				}
			}
		})
	}
}

func BenchmarkCompileNodeAdmission(b *testing.B) {
	large := strings.Repeat("a", 1<<20)
	for _, test := range []struct {
		name      string
		source    string
		options   compileLimits
		wantLimit bool
	}{
		{name: "admitted/literal", source: "abc", options: fixedCompileLimits},
		{name: "admitted/class", source: "[a-z]", options: fixedCompileLimits},
		{name: "admitted/repeat", source: "a*", options: fixedCompileLimits},
		{name: "rejected/low-cap", source: large, options: compileLimits{nodes: 8}.withDefaults(), wantLimit: true},
		{name: "rejected/default-cap", source: large, options: fixedCompileLimits, wantLimit: true},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(test.source)))
			for b.Loop() {
				pattern, err := compileWithLimits(test.source, test.options)
				if test.wantLimit {
					if pattern != nil || !IsLimit(err) {
						b.Fatalf("Compile() = (%v, %v), want nil and limit", pattern, err)
					}
					continue
				}
				if err != nil || pattern == nil {
					b.Fatalf("Compile() = (%v, %v), want pattern and nil error", pattern, err)
				}
			}
		})
	}
}
