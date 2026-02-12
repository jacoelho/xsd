package xmlstream

import (
	"fmt"
	"testing"
)

func TestQNameCacheInternBytesEmptyLocal(t *testing.T) {
	cache := newQNameCache()
	q := cache.internBytes("urn:test", nil)
	if q.Namespace != "urn:test" || q.Local != "" {
		t.Fatalf("QName = %q/%q, want urn:test/empty", q.Namespace, q.Local)
	}
	if cache.recentCount != 1 {
		t.Fatalf("recentCount = %d, want 1", cache.recentCount)
	}
	q = cache.internBytes("urn:test", []byte{})
	if q.Namespace != "urn:test" || q.Local != "" {
		t.Fatalf("QName = %q/%q, want urn:test/empty", q.Namespace, q.Local)
	}
}

func TestQNameCacheInternBytesStable(t *testing.T) {
	cache := newQNameCache()
	buf := []byte("alpha")
	first := cache.internBytes("urn:test", buf)
	if first.Local != "alpha" {
		t.Fatalf("first local = %q, want alpha", first.Local)
	}
	buf[0] = 'o'
	second := cache.internBytes("urn:test", buf)
	if second.Local != string(buf) {
		t.Fatalf("second local = %q, want %q", second.Local, string(buf))
	}
	if second.Local == first.Local {
		t.Fatalf("second local reused stale value %q", second.Local)
	}
}

func TestQNameCacheRecentRingWrap(t *testing.T) {
	cache := newQNameCache()
	for i := range qnameCacheRecentSize + 1 {
		local := fmt.Appendf(nil, "n%d", i)
		cache.internBytes("urn:test", local)
	}
	if cache.recentCount != qnameCacheRecentSize {
		t.Fatalf("recentCount = %d, want %d", cache.recentCount, qnameCacheRecentSize)
	}
	if cache.recentIndex == 0 {
		t.Fatalf("recentIndex = %d, want non-zero after wrap", cache.recentIndex)
	}
}

func TestQNameCacheRecentWrapAtBoundary(t *testing.T) {
	cache := newQNameCache()
	for i := range qnameCacheRecentSize {
		cache.internBytes("urn:test", fmt.Appendf(nil, "n%d", i))
	}
	if cache.recentCount != qnameCacheRecentSize {
		t.Fatalf("recentCount = %d, want %d", cache.recentCount, qnameCacheRecentSize)
	}
	if cache.recentIndex != 0 {
		t.Fatalf("recentIndex = %d, want 0", cache.recentIndex)
	}
	cache.internBytes("urn:test", []byte("n-last"))
	if cache.recentIndex == 0 {
		t.Fatalf("recentIndex = %d, want non-zero after wrap", cache.recentIndex)
	}
}

func TestQNameCacheRecentWrapToZero(t *testing.T) {
	cache := newQNameCache()
	for i := range qnameCacheRecentSize * 2 {
		cache.internBytes("urn:test", fmt.Appendf(nil, "n%d", i))
	}
	if cache.recentIndex != 0 {
		t.Fatalf("recentIndex = %d, want 0 after wrap", cache.recentIndex)
	}
}

func TestQNameCacheRecentHit(t *testing.T) {
	cache := newQNameCache()
	first := cache.internBytes("urn:test", []byte("alpha"))
	if cache.recentCount != 1 {
		t.Fatalf("recentCount = %d, want 1", cache.recentCount)
	}
	second := cache.internBytes("urn:test", []byte("alpha"))
	if first != second {
		t.Fatalf("QName = %v, want %v", second, first)
	}
	if cache.recentCount != 1 {
		t.Fatalf("recentCount = %d, want 1", cache.recentCount)
	}
}

func TestQNameCacheMaxEntries(t *testing.T) {
	cache := newQNameCache()
	total := qnameCacheMaxEntries + qnameCacheRecentSize + 10
	for i := range total {
		cache.internBytes("urn:test", fmt.Appendf(nil, "n%d", i))
	}
	if len(cache.table) > qnameCacheMaxEntries {
		t.Fatalf("table size = %d, want <= %d", len(cache.table), qnameCacheMaxEntries)
	}
}

func TestQNameCacheSetMaxEntriesZero(t *testing.T) {
	cache := newQNameCache()
	cache.setMaxEntries(0)
	for i := range qnameCacheMaxEntries * 2 {
		cache.internBytes("urn:test", fmt.Appendf(nil, "n%d", i))
	}
	if len(cache.table) <= qnameCacheMaxEntries {
		t.Fatalf("table size = %d, want > %d", len(cache.table), qnameCacheMaxEntries)
	}
}

func TestQNameCacheSetMaxEntriesNegative(t *testing.T) {
	cache := newQNameCache()
	cache.setMaxEntries(-5)
	if cache.maxEntries != 0 {
		t.Fatalf("maxEntries = %d, want 0", cache.maxEntries)
	}
}

func TestQNameCacheSetMaxEntriesNil(_ *testing.T) {
	var cache *qnameCache
	cache.setMaxEntries(10)
}

func TestQNameCacheCompactWrappedRing(t *testing.T) {
	cache := newQNameCache()
	cache.setMaxEntries(4)
	total := qnameCacheRecentSize + 4
	for i := range total {
		cache.internBytes("urn:test", fmt.Appendf(nil, "n%d", i))
	}
	if cache.recentIndex == 0 {
		t.Fatalf("recentIndex = 0, want wrapped ring")
	}
	if _, ok := cache.table[qnameKey{namespace: "urn:test", local: "n0"}]; ok {
		t.Fatalf("table contains stale entry n0")
	}
	last := fmt.Sprintf("n%d", total-1)
	if _, ok := cache.table[qnameKey{namespace: "urn:test", local: last}]; !ok {
		t.Fatalf("table missing recent entry %s", last)
	}
	if len(cache.table) > cache.maxEntries {
		t.Fatalf("table size = %d, want <= %d", len(cache.table), cache.maxEntries)
	}
}

func TestQNameCacheCompactNil(_ *testing.T) {
	var cache *qnameCache
	cache.compact()
}

func TestQNameCacheCompactMaxEntriesLessThanRecent(t *testing.T) {
	cache := newQNameCache()
	for i := range qnameCacheRecentSize {
		cache.internBytes("urn:test", fmt.Appendf(nil, "n%d", i))
	}
	cache.setMaxEntries(3)
	if len(cache.table) > 3 {
		t.Fatalf("table size = %d, want <= 3", len(cache.table))
	}
}

func TestQNameCacheEmptyLocalCompactAtLimit(t *testing.T) {
	cache := newQNameCache()
	cache.setMaxEntries(1)
	cache.internBytes("urn:a", nil)
	cache.internBytes("urn:b", nil)
	if len(cache.table) > 1 {
		t.Fatalf("table size = %d, want <= 1", len(cache.table))
	}
}

func TestQNameCacheMapHitNotRecent(t *testing.T) {
	cache := newQNameCache()
	for i := range qnameCacheRecentSize + 1 {
		local := fmt.Appendf(nil, "n%d", i)
		cache.internBytes("urn:test", local)
	}
	cache.internBytes("urn:test", []byte("n0"))
	var found bool
	for i := 0; i < cache.recentCount; i++ {
		if cache.recent[i].local == "n0" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("recent cache missing n0 after map hit")
	}
}

func TestQNameCacheCompactEmpty(t *testing.T) {
	cache := newQNameCache()
	cache.compact()
	if cache.table == nil {
		t.Fatalf("table = nil, want non-nil")
	}
	if len(cache.table) != 0 {
		t.Fatalf("table size = %d, want 0", len(cache.table))
	}
}

func TestQNameCacheEmptyLocalDifferentNamespaces(t *testing.T) {
	cache := newQNameCache()
	first := cache.internBytes("urn:one", nil)
	second := cache.internBytes("urn:two", nil)
	if first.Namespace != "urn:one" || second.Namespace != "urn:two" {
		t.Fatalf("namespaces = %q/%q, want urn:one/urn:two", first.Namespace, second.Namespace)
	}
	if first.Local != "" || second.Local != "" {
		t.Fatalf("locals = %q/%q, want empty", first.Local, second.Local)
	}
	third := cache.internBytes("urn:one", nil)
	if first != third {
		t.Fatalf("empty local cache miss: %v != %v", first, third)
	}
}

func TestQNameCacheEmptyLocalTableHit(t *testing.T) {
	cache := newQNameCache()
	first := cache.internBytes("urn:test", nil)
	for i := range qnameCacheRecentSize + 1 {
		cache.internBytes("urn:test", fmt.Appendf(nil, "name%d", i))
	}
	second := cache.internBytes("urn:test", nil)
	if first != second {
		t.Fatalf("empty local cache miss: %v != %v", first, second)
	}
}

func TestQNameCacheReset(t *testing.T) {
	cache := newQNameCache()
	cache.internBytes("urn:test", []byte("a"))
	cache.reset()
	if len(cache.table) != 0 {
		t.Fatalf("table size = %d, want 0", len(cache.table))
	}
	if cache.recentCount != 0 || cache.recentIndex != 0 {
		t.Fatalf("recentCount/index = %d/%d, want 0/0", cache.recentCount, cache.recentIndex)
	}
}

func TestQNameCacheResetNil(_ *testing.T) {
	var cache *qnameCache
	cache.reset()
}

func TestQNameCacheResetNilTable(t *testing.T) {
	cache := &qnameCache{}
	cache.reset()
	if cache.table == nil {
		t.Fatalf("table = nil, want allocated")
	}
	if len(cache.table) != 0 {
		t.Fatalf("table size = %d, want 0", len(cache.table))
	}
}
