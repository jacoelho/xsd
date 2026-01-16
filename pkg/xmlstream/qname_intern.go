package xmlstream

type qnameCache struct {
	table       map[qnameKey]QName
	recent      [qnameCacheRecentSize]qnameCacheEntry
	recentCount int
	recentIndex int
	maxEntries  int
}

type qnameKey struct {
	namespace string
	local     string
}

const qnameCacheRecentSize = 8
const qnameCacheMaxEntries = 4096

type qnameCacheEntry struct {
	namespace string
	local     string
	qname     QName
}

func newQNameCache() *qnameCache {
	return &qnameCache{
		table:      make(map[qnameKey]QName, 32),
		maxEntries: qnameCacheMaxEntries,
	}
}

func (i *qnameCache) reset() {
	if i == nil {
		return
	}
	if i.table == nil {
		i.table = make(map[qnameKey]QName, 32)
	} else {
		clear(i.table)
	}
	i.recentCount = 0
	i.recentIndex = 0
}

func (i *qnameCache) setMaxEntries(maxEntries int) {
	if i == nil {
		return
	}
	if maxEntries < 0 {
		maxEntries = 0
	}
	i.maxEntries = maxEntries
	if i.maxEntries > 0 && len(i.table) > i.maxEntries {
		i.compact()
	}
}

func (i *qnameCache) lookupRecent(namespace, local string) (QName, bool) {
	for idx := 0; idx < i.recentCount; idx++ {
		entry := i.recent[idx]
		if entry.namespace == namespace && entry.local == local {
			return entry.qname, true
		}
	}
	return QName{}, false
}

func (i *qnameCache) rememberRecent(entry qnameCacheEntry) {
	if i.recentCount < qnameCacheRecentSize {
		i.recent[i.recentCount] = entry
		i.recentCount++
		return
	}
	i.recent[i.recentIndex] = entry
	i.recentIndex++
	if i.recentIndex >= qnameCacheRecentSize {
		i.recentIndex = 0
	}
}

func (i *qnameCache) internBytes(namespace string, local []byte) QName {
	if len(local) == 0 {
		if cached, ok := i.lookupRecent(namespace, ""); ok {
			return cached
		}
		key := qnameKey{namespace: namespace, local: ""}
		if cached, ok := i.table[key]; ok {
			i.rememberRecent(qnameCacheEntry{namespace: namespace, local: "", qname: cached})
			return cached
		}
		qname := QName{Namespace: namespace, Local: ""}
		i.table[key] = qname
		i.rememberRecent(qnameCacheEntry{namespace: namespace, local: "", qname: qname})
		if i.maxEntries > 0 && len(i.table) > i.maxEntries {
			i.compact()
		}
		return qname
	}
	localKey := unsafeString(local)
	// localKey is only used for lookup; stable strings are stored in the cache.
	if cached, ok := i.lookupRecent(namespace, localKey); ok {
		return cached
	}
	key := qnameKey{namespace: namespace, local: localKey}
	if cached, ok := i.table[key]; ok {
		i.rememberRecent(qnameCacheEntry{namespace: namespace, local: cached.Local, qname: cached})
		return cached
	}
	localStable := string(local)
	qname := QName{Namespace: namespace, Local: localStable}
	i.table[qnameKey{namespace: namespace, local: localStable}] = qname
	i.rememberRecent(qnameCacheEntry{namespace: namespace, local: localStable, qname: qname})
	if i.maxEntries > 0 && len(i.table) > i.maxEntries {
		i.compact()
	}
	return qname
}

func (i *qnameCache) compact() {
	if i == nil {
		return
	}
	limit := i.recentCount
	if i.maxEntries > 0 && i.maxEntries < limit {
		limit = i.maxEntries
	}
	next := make(map[qnameKey]QName, limit)
	// walk the ring from most-recent to oldest to keep the hottest entries.
	for offset := 0; offset < limit; offset++ {
		var idx int
		if i.recentCount < qnameCacheRecentSize {
			idx = i.recentCount - 1 - offset
		} else {
			idx = i.recentIndex - 1 - offset
			if idx < 0 {
				idx += i.recentCount
			}
		}
		entry := i.recent[idx]
		next[qnameKey{namespace: entry.namespace, local: entry.local}] = entry.qname
	}
	i.table = next
}
