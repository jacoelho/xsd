package xmlstream

import (
	"strconv"
	"strings"
	"testing"
)

func benchmarkSaturatedCache() (*cache, [][]byte) {
	c := &cache{}
	for i := range maxByteStringCacheEntries {
		c.Intern([]byte("map-" + strconv.Itoa(i)))
	}
	misses := make([][]byte, 9)
	for i := range misses {
		misses[i] = []byte("miss-" + strconv.Itoa(i))
	}
	return c, misses
}

func BenchmarkCacheSaturation(b *testing.B) {
	b.Run("cold-short", func(b *testing.B) {
		input := []byte("cold-short")
		b.ReportAllocs()
		for b.Loop() {
			var c cache
			c.Intern(input)
		}
	})

	b.Run("warm-short", func(b *testing.B) {
		var c cache
		input := []byte("warm-short")
		c.Intern(input)
		b.ReportAllocs()
		for b.Loop() {
			c.Intern(input)
		}
	})

	b.Run("saturated-hot", func(b *testing.B) {
		c, misses := benchmarkSaturatedCache()
		input := misses[0]
		c.Intern(input)
		b.ReportAllocs()
		for b.Loop() {
			c.Intern(input)
		}
	})

	b.Run("saturated-9-way", func(b *testing.B) {
		c, misses := benchmarkSaturatedCache()
		for _, input := range misses {
			c.Intern(input)
		}
		b.ReportAllocs()
		for i := 0; b.Loop(); i++ {
			c.Intern(misses[i%len(misses)])
		}
	})

	b.Run("saturated-long", func(b *testing.B) {
		c, _ := benchmarkSaturatedCache()
		input := []byte(strings.Repeat("l", maxByteStringCacheLen+1))
		b.ReportAllocs()
		for b.Loop() {
			c.Intern(input)
		}
	})
}

// BenchmarkCacheRotatingVocabulary cycles through vocabularies of several
// widths. The middle shapes use names longer than eight bytes that differ only
// in their middle bytes.
func BenchmarkCacheRotatingVocabulary(b *testing.B) {
	for _, shape := range []struct {
		name  string
		width int
		spell func(int) string
	}{
		{"2", 2, rotatingName},
		{"9", 9, rotatingName},
		{"60", 60, rotatingName},
		{"middle-8", 8, middleName},
		{"middle-60", 60, middleName},
	} {
		b.Run(shape.name, func(b *testing.B) {
			var c cache
			spellings := make([][]byte, shape.width)
			for i := range spellings {
				spellings[i] = []byte(shape.spell(i))
				if got := c.Intern(spellings[i]); got != string(spellings[i]) {
					b.Fatalf("cached spelling = %q, want %q", got, spellings[i])
				}
			}
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				c.Intern(spellings[i%len(spellings)])
			}
		})
	}
}

func rotatingName(i int) string { return "name-" + strconv.Itoa(i) }

func middleName(i int) string { return "address_" + strconv.Itoa(i) + "_city" }
