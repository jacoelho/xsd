package xmlstream

import (
	"strconv"
	"strings"
	"testing"
)

func fillStringCache(t *testing.T, c *cache) {
	t.Helper()
	for i := range maxByteStringCacheEntries {
		input := []byte("map-" + strconv.Itoa(i))
		if got := c.Intern(input); got != string(input) {
			t.Fatalf("map fill %q = %q", input, got)
		}
	}
}

func TestCacheSaturatedShortMissRemainsOwnedAndBounded(t *testing.T) {
	var c cache
	fillStringCache(t, &c)
	miss := []byte("saturated-miss")
	owned := c.Intern(miss)
	miss[0] = 'X'
	if owned != "saturated-miss" {
		t.Fatalf("saturated miss ownership = %q, want saturated-miss", owned)
	}
	if got := len(c.state.interned); got != maxByteStringCacheEntries {
		t.Fatalf("interned entries = %d, want %d", got, maxByteStringCacheEntries)
	}
}

func TestCacheSaturatedNineWayMissesRemainBounded(t *testing.T) {
	var c cache
	fillStringCache(t, &c)
	for i := range 9 {
		spelling := "miss-" + strconv.Itoa(i)
		input := []byte(spelling)
		owned := c.Intern(input)
		input[0] = 'X'
		if owned != spelling {
			t.Fatalf("miss ownership = %q, want %q", owned, spelling)
		}
	}
	if got := len(c.state.interned); got != maxByteStringCacheEntries {
		t.Fatalf("interned entries after saturation = %d, want %d", got, maxByteStringCacheEntries)
	}
	for _, i := range []int{0, 255, 511} {
		spelling := "map-" + strconv.Itoa(i)
		if got := c.Intern([]byte(spelling)); got != spelling {
			t.Fatalf("cached spelling after saturation = %q, want %q", got, spelling)
		}
	}
}

func TestCacheSaturatedLongMissIsNotRetained(t *testing.T) {
	var c cache
	fillStringCache(t, &c)
	long := []byte(strings.Repeat("l", maxByteStringCacheLen+1))
	owned := c.Intern(long)
	long[0] = 'X'
	if owned != strings.Repeat("l", maxByteStringCacheLen+1) {
		t.Fatalf("long miss lost its owned spelling: %q", owned)
	}
	if got := len(c.state.interned); got != maxByteStringCacheEntries {
		t.Fatalf("interned entries after long miss = %d, want %d", got, maxByteStringCacheEntries)
	}
}

func TestCacheSpellingLengthRetentionBoundary(t *testing.T) {
	for _, size := range []int{256, 257} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			var c cache
			spelling := strings.Repeat("n", size)
			input := []byte(spelling)
			owned := c.Intern(input)
			input[0] = 'X'
			if owned != spelling {
				t.Fatal("input mutation changed the owned spelling")
			}
			want := 0
			if size == 256 {
				want = 1
			}
			if got := len(c.state.interned); got != want {
				t.Fatalf("retained spellings = %d, want %d", got, want)
			}
			if got := c.Intern([]byte(spelling)); got != spelling {
				t.Fatalf("repeated lookup = %q, want original spelling", got)
			}
		})
	}
}
