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
