package xmlstream

type nameCache struct {
	table       map[qnameKey]nameCacheEntry
	recent      [qnameCacheRecentSize]nameCacheEntry
	recentCount int
	recentIndex int
	nextID      NameID
}

type nameCacheEntry struct {
	namespace string
	local     string
	id        NameID
}

func newNameCache() *nameCache {
	return &nameCache{
		table: make(map[qnameKey]nameCacheEntry, 32),
	}
}

func (c *nameCache) reset() {
	if c == nil {
		return
	}
	if c.table == nil {
		c.table = make(map[qnameKey]nameCacheEntry, 32)
	} else {
		clear(c.table)
	}
	c.recentCount = 0
	c.recentIndex = 0
	c.nextID = 0
}

func (c *nameCache) lookupRecent(namespace, local string) (NameID, bool) {
	for idx := 0; idx < c.recentCount; idx++ {
		entry := c.recent[idx]
		if entry.namespace == namespace && entry.local == local {
			return entry.id, true
		}
	}
	return 0, false
}

func (c *nameCache) rememberRecent(entry nameCacheEntry) {
	if c.recentCount < qnameCacheRecentSize {
		c.recent[c.recentCount] = entry
		c.recentCount++
		return
	}
	c.recent[c.recentIndex] = entry
	c.recentIndex++
	if c.recentIndex >= qnameCacheRecentSize {
		c.recentIndex = 0
	}
}

func (c *nameCache) internBytes(namespace string, local []byte) NameID {
	if c == nil {
		return 0
	}
	if c.table == nil {
		c.table = make(map[qnameKey]nameCacheEntry, 32)
	}
	if len(local) == 0 {
		if id, ok := c.lookupRecent(namespace, ""); ok {
			return id
		}
		key := qnameKey{namespace: namespace, local: ""}
		if entry, ok := c.table[key]; ok {
			c.rememberRecent(entry)
			return entry.id
		}
		c.nextID++
		entry := nameCacheEntry{namespace: namespace, local: "", id: c.nextID}
		c.table[key] = entry
		c.rememberRecent(entry)
		return entry.id
	}
	localKey := unsafeString(local)
	if id, ok := c.lookupRecent(namespace, localKey); ok {
		return id
	}
	key := qnameKey{namespace: namespace, local: localKey}
	if entry, ok := c.table[key]; ok {
		c.rememberRecent(entry)
		return entry.id
	}
	localStable := string(local)
	key = qnameKey{namespace: namespace, local: localStable}
	c.nextID++
	id := c.nextID
	entry := nameCacheEntry{namespace: namespace, local: localStable, id: id}
	c.table[key] = entry
	c.rememberRecent(entry)
	return entry.id
}
