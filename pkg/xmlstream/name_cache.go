package xmlstream

type resolvedNameCache struct {
	table       map[qnameKey]resolvedNameEntry
	recent      [qnameCacheRecentSize]resolvedNameEntry
	recentCount int
	recentIndex int
	nextID      NameID
}

type resolvedNameEntry struct {
	qname QName
	id    NameID
}

func newResolvedNameCache() *resolvedNameCache {
	return &resolvedNameCache{
		table: make(map[qnameKey]resolvedNameEntry, 32),
	}
}

func (c *resolvedNameCache) reset() {
	if c == nil {
		return
	}
	if c.table == nil {
		c.table = make(map[qnameKey]resolvedNameEntry, 32)
	} else {
		clear(c.table)
	}
	c.recentCount = 0
	c.recentIndex = 0
	c.nextID = 0
}

func (c *resolvedNameCache) lookupRecent(namespace, local string) (resolvedNameEntry, bool) {
	for idx := 0; idx < c.recentCount; idx++ {
		entry := c.recent[idx]
		if entry.qname.Namespace == namespace && entry.qname.Local == local {
			return entry, true
		}
	}
	return resolvedNameEntry{}, false
}

func (c *resolvedNameCache) rememberRecent(entry resolvedNameEntry) {
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

func (c *resolvedNameCache) internBytes(namespace string, local []byte) resolvedNameEntry {
	if c == nil {
		return resolvedNameEntry{}
	}
	if c.table == nil {
		c.table = make(map[qnameKey]resolvedNameEntry, 32)
	}
	if len(local) == 0 {
		if entry, ok := c.lookupRecent(namespace, ""); ok {
			return entry
		}
		key := qnameKey{namespace: namespace, local: ""}
		if entry, ok := c.table[key]; ok {
			c.rememberRecent(entry)
			return entry
		}
		c.nextID++
		entry := resolvedNameEntry{
			qname: QName{Namespace: namespace, Local: ""},
			id:    c.nextID,
		}
		c.table[key] = entry
		c.rememberRecent(entry)
		return entry
	}
	localKey := unsafeString(local)
	if entry, ok := c.lookupRecent(namespace, localKey); ok {
		return entry
	}
	key := qnameKey{namespace: namespace, local: localKey}
	if entry, ok := c.table[key]; ok {
		c.rememberRecent(entry)
		return entry
	}
	localStable := string(local)
	c.nextID++
	entry := resolvedNameEntry{
		qname: QName{Namespace: namespace, Local: localStable},
		id:    c.nextID,
	}
	c.table[qnameKey{namespace: namespace, local: localStable}] = entry
	c.rememberRecent(entry)
	return entry
}
