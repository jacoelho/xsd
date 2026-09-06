package valuebench

import (
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
