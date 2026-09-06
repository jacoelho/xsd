package xsdregex

import (
	"strings"
	"testing"
)

func BenchmarkSimplePatternVariableNoMatchString(b *testing.B) {
	pattern := benchmarkPattern(b, `[a-z]{0,}[a-z]{0,}x`)
	input := strings.Repeat("a", 4096)
	b.ReportAllocs()
	for b.Loop() {
		matched, err := pattern.MatchString(input)
		if err != nil {
			b.Fatal(err)
		}
		if matched {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchBytes(b *testing.B) {
	pattern := benchmarkPattern(b, `[a-z]{0,}[a-z]{0,}x`)
	input := []byte(strings.Repeat("a", 4096))
	b.ReportAllocs()
	for b.Loop() {
		matched, err := pattern.MatchBytes(input)
		if err != nil {
			b.Fatal(err)
		}
		if matched {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchStringScratch(b *testing.B) {
	pattern := benchmarkPattern(b, `[a-z]{0,}[a-z]{0,}x`)
	input := strings.Repeat("a", 4096)
	var scratch Scratch
	matchBenchmarkStringScratch(b, pattern, input, &scratch)
}

func BenchmarkSimplePatternVariableNoMatchBytesScratch(b *testing.B) {
	pattern := benchmarkPattern(b, `[a-z]{0,}[a-z]{0,}x`)
	input := []byte(strings.Repeat("a", 4096))
	var scratch Scratch
	matchBenchmarkBytesScratch(b, pattern, input, &scratch)
}

func BenchmarkSimplePatternVariableSmallString(b *testing.B) {
	pattern := benchmarkPattern(b, `[a-z]{0,}[a-z]{0,}x`)
	input := strings.Repeat("a", 24) + "x"
	b.ReportAllocs()
	for b.Loop() {
		matched, err := pattern.MatchString(input)
		if err != nil {
			b.Fatal(err)
		}
		if !matched {
			b.Fatal("expected match")
		}
	}
}

func BenchmarkSimplePatternVariableSmallBytes(b *testing.B) {
	pattern := benchmarkPattern(b, `[a-z]{0,}[a-z]{0,}x`)
	input := []byte(strings.Repeat("a", 24) + "x")
	b.ReportAllocs()
	for b.Loop() {
		matched, err := pattern.MatchBytes(input)
		if err != nil {
			b.Fatal(err)
		}
		if !matched {
			b.Fatal("expected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchMultibyteString(b *testing.B) {
	pattern := benchmarkPattern(b, `é{0,}é{0,}x`)
	input := strings.Repeat("é", 4096)
	b.ReportAllocs()
	for b.Loop() {
		matched, err := pattern.MatchString(input)
		if err != nil {
			b.Fatal(err)
		}
		if matched {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchMultibyteBytes(b *testing.B) {
	pattern := benchmarkPattern(b, `é{0,}é{0,}x`)
	input := []byte(strings.Repeat("é", 4096))
	b.ReportAllocs()
	for b.Loop() {
		matched, err := pattern.MatchBytes(input)
		if err != nil {
			b.Fatal(err)
		}
		if matched {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchMultibyteStringScratch(b *testing.B) {
	pattern := benchmarkPattern(b, `é{0,}é{0,}x`)
	input := strings.Repeat("é", 4096)
	var scratch Scratch
	matchBenchmarkStringScratch(b, pattern, input, &scratch)
}

func BenchmarkSimplePatternVariableNoMatchMultibyteBytesScratch(b *testing.B) {
	pattern := benchmarkPattern(b, `é{0,}é{0,}x`)
	input := []byte(strings.Repeat("é", 4096))
	var scratch Scratch
	matchBenchmarkBytesScratch(b, pattern, input, &scratch)
}

func benchmarkPattern(b *testing.B, source string) *Pattern {
	b.Helper()
	pattern, err := Compile(source, CompileOptions{})
	if err != nil {
		b.Fatal(err)
	}
	return pattern
}

func matchBenchmarkStringScratch(b *testing.B, pattern *Pattern, input string, scratch *Scratch) {
	b.Helper()
	matched, err := pattern.MatchStringWithScratch(input, MatchOptions{}, scratch)
	if err != nil {
		b.Fatal(err)
	}
	if matched {
		b.Fatal("unexpected match")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		matched, err := pattern.MatchStringWithScratch(input, MatchOptions{}, scratch)
		if err != nil {
			b.Fatal(err)
		}
		if matched {
			b.Fatal("unexpected match")
		}
	}
}

func matchBenchmarkBytesScratch(b *testing.B, pattern *Pattern, input []byte, scratch *Scratch) {
	b.Helper()
	matched, err := pattern.MatchBytesWithScratch(input, MatchOptions{}, scratch)
	if err != nil {
		b.Fatal(err)
	}
	if matched {
		b.Fatal("unexpected match")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		matched, err := pattern.MatchBytesWithScratch(input, MatchOptions{}, scratch)
		if err != nil {
			b.Fatal(err)
		}
		if matched {
			b.Fatal("unexpected match")
		}
	}
}
