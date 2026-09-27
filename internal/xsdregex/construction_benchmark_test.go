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
				pattern, err := Compile(source, CompileOptions{})
				if err != nil {
					b.Fatal(err)
				}
				matched, err := pattern.MatchString("A")
				if err != nil || !matched {
					b.Fatalf("MatchString(A) = %v, %v", matched, err)
				}
			}
		})
	}
}

func BenchmarkParseNodeAdmissionPrebuilt(b *testing.B) {
	options := normalizeCompileOptions(CompileOptions{MaxNodes: 8})
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
		options   CompileOptions
		wantLimit bool
	}{
		{name: "admitted/literal", source: "abc", options: CompileOptions{}},
		{name: "admitted/class", source: "[a-z]", options: CompileOptions{}},
		{name: "admitted/repeat", source: "a*", options: CompileOptions{}},
		{name: "rejected/low-cap", source: large, options: CompileOptions{MaxNodes: 8}, wantLimit: true},
		{name: "rejected/default-cap", source: large, options: CompileOptions{}, wantLimit: true},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(test.source)))
			for b.Loop() {
				pattern, err := Compile(test.source, test.options)
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
