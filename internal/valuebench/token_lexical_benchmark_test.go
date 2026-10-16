package valuebench

import (
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

const longTokenPrefix = 4096

type tokenNamedString string
type tokenNamedBytes []byte

type tokenBooleanExpectation struct {
	value   bool
	message string
}

func TestLexicalTokenComparisonBenchmarkPreflight(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw      string
		expected tokenBooleanExpectation
	}{
		{"true", tokenBooleanExpectation{value: true}},
		{"false", tokenBooleanExpectation{}},
		{"falsx", tokenBooleanExpectation{message: "invalid boolean"}},
	} {
		checkTokenBoolean(t, tc.raw, tc.expected)
		checkTokenBoolean(t, []byte(tc.raw), tc.expected)
		checkTokenBoolean(t, tokenNamedString(tc.raw), tc.expected)
		checkTokenBoolean(t, tokenNamedBytes(tc.raw), tc.expected)
	}
	for _, tc := range []struct {
		raw     string
		bits    int
		message string
	}{
		{"INF", 64, ""}, {"-INF", 64, ""}, {"NaN", 64, ""},
		{"1.234567E+10", 32, ""}, {"-1.23456789012345E-20", 64, ""}, {"INX", 64, "invalid float"},
		{strings.Repeat("1", longTokenPrefix) + "X", 64, "invalid float"},
		{"X" + strings.Repeat("1", longTokenPrefix), 64, "invalid float"},
	} {
		checkTokenFloat(t, tc.raw, tc.bits, tc.message)
		checkTokenFloat(t, []byte(tc.raw), tc.bits, tc.message)
		checkTokenFloat(t, tokenNamedString(tc.raw), tc.bits, tc.message)
		checkTokenFloat(t, tokenNamedBytes(tc.raw), tc.bits, tc.message)
	}
}

func checkTokenBoolean[T interface{ ~string | ~[]byte }](t *testing.T, raw T, expected tokenBooleanExpectation) {
	t.Helper()
	got, err := valuepkg.ParseBooleanValue(raw)
	if got != expected.value || (err == nil) != (expected.message == "") || err != nil && err.Error() != expected.message {
		t.Fatalf("ParseBooleanValue(%q)=(%t,%v), want(%t,error=%q)", raw, got, err, expected.value, expected.message)
	}
}

func checkTokenFloat[T interface{ ~string | ~[]byte }](t *testing.T, raw T, bits int, message string) {
	t.Helper()
	if err := valuepkg.ValidateFloatLexical(raw, bits); (err == nil) != (message == "") || err != nil && err.Error() != message {
		t.Fatalf("ValidateFloatLexical(%q,%d)=%v, want error=%q", raw, bits, err, message)
	}
}

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
