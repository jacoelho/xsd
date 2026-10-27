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

func BenchmarkCacheRotatingVocabulary(b *testing.B) {
	for _, width := range []int{2, 9, 60} {
		b.Run(strconv.Itoa(width), func(b *testing.B) {
			var c cache
			spellings := make([][]byte, width)
			for i := range spellings {
				spellings[i] = []byte("name-" + strconv.Itoa(i))
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
