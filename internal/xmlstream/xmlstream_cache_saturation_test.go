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

func nonEmptyRecentEntries(c *cache) int {
	count := 0
	for _, s := range c.state.recent {
		if s != "" {
			count++
		}
	}
	return count
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
	if got := nonEmptyRecentEntries(&c); got > recentCacheEntries {
		t.Fatalf("recent entries = %d, want at most %d", got, recentCacheEntries)
	}
}

func TestCacheSaturatedNineWayMissesRemainBounded(t *testing.T) {
	var c cache
	fillStringCache(t, &c)
	for i := range recentCacheEntries + 1 {
		c.Intern([]byte("miss-" + strconv.Itoa(i)))
	}
	if got := len(c.state.interned); got != maxByteStringCacheEntries {
		t.Fatalf("interned entries after saturation = %d, want %d", got, maxByteStringCacheEntries)
	}
	if got := nonEmptyRecentEntries(&c); got != recentCacheEntries {
		t.Fatalf("recent entries after nine misses = %d, want %d", got, recentCacheEntries)
	}
}

func TestCacheSaturatedLongMissIsNotRetained(t *testing.T) {
	var c cache
	fillStringCache(t, &c)
	before := nonEmptyRecentEntries(&c)
	long := []byte(strings.Repeat("l", maxByteStringCacheLen+1))
	if got := c.Intern(long); got != string(long) {
		t.Fatalf("long miss = %q, want input spelling", got)
	}
	if got := len(c.state.interned); got != maxByteStringCacheEntries {
		t.Fatalf("interned entries after long miss = %d, want %d", got, maxByteStringCacheEntries)
	}
	if got := nonEmptyRecentEntries(&c); got != before {
		t.Fatalf("recent entries after long miss = %d, want %d", got, before)
	}
}
