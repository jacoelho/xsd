package runtime

import (
	"strings"
	"testing"
)

func BenchmarkSimplePatternVariableNoMatchString(b *testing.B) {
	p := CompileSimpleStringPattern(`[a-z]{0,}[a-z]{0,}x`)
	input := strings.Repeat("a", 4096)
	b.ReportAllocs()
	for b.Loop() {
		if p.MatchString(input) {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchBytes(b *testing.B) {
	p := CompileSimpleStringPattern(`[a-z]{0,}[a-z]{0,}x`)
	input := []byte(strings.Repeat("a", 4096))
	b.ReportAllocs()
	for b.Loop() {
		if p.MatchBytes(input) {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableSmallString(b *testing.B) {
	p := CompileSimpleStringPattern(`[a-z]{0,}[a-z]{0,}x`)
	input := strings.Repeat("a", 24) + "x"
	b.ReportAllocs()
	for b.Loop() {
		if !p.MatchString(input) {
			b.Fatal("expected match")
		}
	}
}

func BenchmarkSimplePatternVariableSmallBytes(b *testing.B) {
	p := CompileSimpleStringPattern(`[a-z]{0,}[a-z]{0,}x`)
	input := []byte(strings.Repeat("a", 24) + "x")
	b.ReportAllocs()
	for b.Loop() {
		if !p.MatchBytes(input) {
			b.Fatal("expected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchMultibyteString(b *testing.B) {
	p := CompileSimpleStringPattern(`é{0,}é{0,}x`)
	input := strings.Repeat("é", 4096)
	b.ReportAllocs()
	for b.Loop() {
		if p.MatchString(input) {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchMultibyteBytes(b *testing.B) {
	p := CompileSimpleStringPattern(`é{0,}é{0,}x`)
	input := []byte(strings.Repeat("é", 4096))
	b.ReportAllocs()
	for b.Loop() {
		if p.MatchBytes(input) {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkParseDecimal(b *testing.B) {
	for b.Loop() {
		if _, err := ParseDecimalCanonical("+000000000123456789.0000000012300"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseXSDDate(b *testing.B) {
	for b.Loop() {
		if _, err := ParseDateValue("12026-05-18+14:00"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseXSDDateTime(b *testing.B) {
	for b.Loop() {
		if _, err := ParseDateTimeValue("-12026-05-18T23:59:59.123456789123+14:00"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseXSDTime(b *testing.B) {
	for b.Loop() {
		if _, err := ParseTimeValue("23:59:60.123456789123-14:00"); err != nil {
			b.Fatal(err)
		}
	}
}
