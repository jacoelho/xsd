package xsdregex

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkBitsetMatch(b *testing.B) {
	for _, shape := range []struct {
		name, pattern, unit, suffix string
	}{
		{"sparse", `(ab|ac)*z`, "ab", "z"},
		{"dense", `(a|aa|aaa|aaaa|aaaaa|aaaaaa)*b`, "a", "b"},
	} {
		for _, length := range []int{16, 4096} {
			b.Run(fmt.Sprintf("%s/%d", shape.name, length), func(b *testing.B) {
				pattern := benchmarkPattern(b, shape.pattern)
				if !pattern.bitset || pattern.deterministic || pattern.linear != nil {
					b.Fatal("benchmark requires the general bitset NFA")
				}
				input := strings.Repeat(shape.unit, length/len(shape.unit)) + shape.suffix
				raw := []byte(input)
				b.Run("string", func(b *testing.B) {
					var scratch Scratch
					b.ReportAllocs()
					b.SetBytes(int64(len(input)))
					for b.Loop() {
						matched, err := pattern.MatchStringWithScratch(input, MatchOptions{}, &scratch)
						if err != nil || !matched {
							b.Fatalf("match = %v, %v", matched, err)
						}
					}
				})
				b.Run("bytes", func(b *testing.B) {
					var scratch Scratch
					b.ReportAllocs()
					b.SetBytes(int64(len(raw)))
					for b.Loop() {
						matched, err := pattern.MatchBytesWithScratch(raw, MatchOptions{}, &scratch)
						if err != nil || !matched {
							b.Fatalf("match = %v, %v", matched, err)
						}
					}
				})
			})
		}
	}
}
