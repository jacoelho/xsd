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

func BenchmarkSimplePatternVariableNoMatchStringScratch(b *testing.B) {
	read := stringPatternRead{fast: CompileSimpleStringPattern(`[a-z]{0,}[a-z]{0,}x`)}
	input := strings.Repeat("a", 4096)
	var scratch StringPatternScratch
	prepared := simplePatternInput{text: input}
	read.matchStringWithScratch(input, &prepared, &scratch)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		prepared = simplePatternInput{text: input}
		if read.matchStringWithScratch(input, &prepared, &scratch) {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchBytesScratch(b *testing.B) {
	read := stringPatternRead{fast: CompileSimpleStringPattern(`[a-z]{0,}[a-z]{0,}x`)}
	input := []byte(strings.Repeat("a", 4096))
	var scratch StringPatternScratch
	prepared := simplePatternInput{bytes: input}
	read.matchBytesWithScratch(input, &prepared, &scratch)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		prepared = simplePatternInput{bytes: input}
		if read.matchBytesWithScratch(input, &prepared, &scratch) {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchGroupScratch(b *testing.B) {
	pattern := CompileSimpleStringPattern(`[a-z]{0,}[a-z]{0,}x`)
	patterns := make([]stringPatternRead, 8)
	for i := range patterns {
		patterns[i] = stringPatternRead{fast: pattern}
	}
	step := &stringPatternStepRead{patterns: patterns}
	input := strings.Repeat("a", 4096)
	var scratch StringPatternScratch
	if err := validateStringPatternStepReadsWithScratch(step, input, &scratch); err == nil {
		b.Fatal("unexpected match")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := validateStringPatternStepReadsWithScratch(step, input, &scratch); err == nil {
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

func BenchmarkSimplePatternVariableNoMatchMultibyteStringScratch(b *testing.B) {
	read := stringPatternRead{fast: CompileSimpleStringPattern(`é{0,}é{0,}x`)}
	input := strings.Repeat("é", 4096)
	var scratch StringPatternScratch
	prepared := simplePatternInput{text: input}
	read.matchStringWithScratch(input, &prepared, &scratch)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		prepared = simplePatternInput{text: input}
		if read.matchStringWithScratch(input, &prepared, &scratch) {
			b.Fatal("unexpected match")
		}
	}
}

func BenchmarkSimplePatternVariableNoMatchMultibyteBytesScratch(b *testing.B) {
	read := stringPatternRead{fast: CompileSimpleStringPattern(`é{0,}é{0,}x`)}
	input := []byte(strings.Repeat("é", 4096))
	var scratch StringPatternScratch
	prepared := simplePatternInput{bytes: input}
	read.matchBytesWithScratch(input, &prepared, &scratch)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		prepared = simplePatternInput{bytes: input}
		if read.matchBytesWithScratch(input, &prepared, &scratch) {
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

func BenchmarkPublishedRawUnionLateMember(b *testing.B) {
	types := []SimpleType{
		{Union: []SimpleTypeID{1, 2}, Variety: SimpleVarietyUnion},
		{Variety: SimpleVarietyAtomic, Primitive: PrimitiveBoolean},
		{Variety: SimpleVarietyAtomic, Primitive: PrimitiveString},
	}
	schema := &Schema{runtime: schemaRuntime{
		SimpleValueRoutes: newSimpleValueRouteReadsForSimpleTypes(types),
		SimpleTypeCold:    newSimpleTypeColdReadTable(types),
	}}
	raw := []byte("value")
	b.ReportAllocs()
	for b.Loop() {
		handled, err := schema.validatePublishedRawSimpleValueWithScratch(0, raw, nil)
		if err != nil || !handled {
			b.Fatalf("validatePublishedRawSimpleValue() = %v, %v; want true, nil", handled, err)
		}
	}
}
