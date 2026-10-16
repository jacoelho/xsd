package valuebench

import (
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func BenchmarkParseDecimal(b *testing.B) {
	for b.Loop() {
		if _, err := valuepkg.ParseDecimalCanonical("+000000000123456789.0000000012300"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseXSDDate(b *testing.B) {
	for b.Loop() {
		if _, err := valuepkg.ParseDateValue("12026-05-18+14:00"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseXSDDateTime(b *testing.B) {
	for b.Loop() {
		if _, err := valuepkg.ParseDateTimeValue("-12026-05-18T23:59:59.123456789123+14:00"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseXSDTime(b *testing.B) {
	for b.Loop() {
		if _, err := valuepkg.ParseTimeValue("23:59:60.123456789123-14:00"); err != nil {
			b.Fatal(err)
		}
	}
}

const longTokenPrefix = 4096

func BenchmarkLexicalTokenComparisons(b *testing.B) {
	for _, tc := range []struct {
		name, raw     string
		want, invalid bool
	}{
		{"BooleanBytesTrue", "true", true, false},
		{"BooleanBytesFalse", "false", false, false},
		{"BooleanBytesMalformed", "falsx", false, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			raw := []byte(tc.raw)
			b.ReportAllocs()
			for b.Loop() {
				got, err := valuepkg.ParseBooleanValue(raw)
				if got != tc.want || (err != nil) != tc.invalid {
					b.Fatalf("unexpected boolean result: %t,%v", got, err)
				}
			}
			if tc.invalid {
				b.ReportMetric(1, "errors/op")
			}
		})
	}
	for _, tc := range []struct {
		name, raw string
		bits      int
		invalid   bool
	}{
		{"FloatBytesINF", "INF", 64, false},
		{"FloatBytesNegINF", "-INF", 64, false},
		{"FloatBytesNaN", "NaN", 64, false},
		{"FloatBytesFinite32", "1.234567E+10", 32, false},
		{"FloatBytesFinite64", "-1.23456789012345E-20", 64, false},
		{"FloatBytesMalformedShort", "INX", 64, true},
		{"FloatBytesMalformedLong", strings.Repeat("1", longTokenPrefix) + "X", 64, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			raw := []byte(tc.raw)
			b.ReportAllocs()
			for b.Loop() {
				if err := valuepkg.ValidateFloatLexical(raw, tc.bits); (err != nil) != tc.invalid {
					b.Fatalf("unexpected float result: %v", err)
				}
			}
			if tc.invalid {
				b.ReportMetric(1, "errors/op")
			}
		})
	}
	b.Run("BooleanStringTokens", func(b *testing.B) {
		tokens := [...]string{"true", "false"}
		expected := [...]bool{true, false}
		i := 0
		b.ReportAllocs()
		for b.Loop() {
			got, err := valuepkg.ParseBooleanValue(tokens[i])
			if err != nil || got != expected[i] {
				b.Fatalf("unexpected boolean result: %t,%v", got, err)
			}
			i++
			if i == len(tokens) {
				i = 0
			}
		}
	})
	b.Run("FloatStringSpecials", func(b *testing.B) {
		tokens := [...]string{"INF", "-INF", "NaN"}
		i := 0
		b.ReportAllocs()
		for b.Loop() {
			if err := valuepkg.ValidateFloatLexical(tokens[i], 64); err != nil {
				b.Fatal(err)
			}
			i++
			if i == len(tokens) {
				i = 0
			}
		}
	})
}
