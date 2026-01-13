package xmltext

import (
	"bytes"
	"hash/maphash"
)

type internStats struct {
	Count  int
	Hits   int
	Misses int
}

const nameInternerRecentSize = 8

var hashSeed = maphash.MakeSeed()

type internEntry struct {
	name qnameSpan
	hash uint64
}

type nameInterner struct {
	buf          spanBuffer
	entries      map[uint64][]internEntry
	recentHashes [nameInternerRecentSize]uint64
	recentNames  [nameInternerRecentSize]qnameSpan
	stats        internStats
	maxEntries   int
	recentCount  int
	recentIndex  int
}

func newNameInterner(maxEntries int) *nameInterner {
	interner := &nameInterner{
		entries:    make(map[uint64][]internEntry, 64),
		maxEntries: maxEntries,
	}
	interner.buf.stable = true
	return interner
}

func (i *nameInterner) setMax(maxEntries int) {
	i.maxEntries = maxEntries
}

func (i *nameInterner) intern(name []byte) qnameSpan {
	return i.internBytes(name, -1)
}

func (i *nameInterner) internQName(name qnameSpan) qnameSpan {
	data := name.Full.bytesUnsafe()
	if len(data) == 0 {
		return qnameSpan{}
	}
	prefixLen := -1
	if name.HasPrefix {
		prefixLen = name.Prefix.End - name.Prefix.Start
	}
	return i.internBytes(data, prefixLen)
}

func (i *nameInterner) internQNameHash(name qnameSpan, hash uint64) qnameSpan {
	data := name.Full.bytesUnsafe()
	if len(data) == 0 {
		return qnameSpan{}
	}
	prefixLen := -1
	if name.HasPrefix {
		prefixLen = name.Prefix.End - name.Prefix.Start
	}
	return i.internBytesHash(data, prefixLen, hash)
}

func (i *nameInterner) internBytes(name []byte, prefixLen int) qnameSpan {
	if len(name) == 0 {
		return qnameSpan{}
	}
	hash := hashBytes(name)
	return i.internBytesHash(name, prefixLen, hash)
}

func (i *nameInterner) lookupRecent(name []byte, hash uint64) (qnameSpan, bool) {
	for idx := 0; idx < i.recentCount; idx++ {
		if i.recentHashes[idx] != hash {
			continue
		}
		if bytes.Equal(i.recentNames[idx].Full.bytesUnsafe(), name) {
			return i.recentNames[idx], true
		}
	}
	return qnameSpan{}, false
}

func (i *nameInterner) rememberRecent(name qnameSpan, hash uint64) {
	if i.recentCount < nameInternerRecentSize {
		i.recentHashes[i.recentCount] = hash
		i.recentNames[i.recentCount] = name
		i.recentCount++
		return
	}
	i.recentHashes[i.recentIndex] = hash
	i.recentNames[i.recentIndex] = name
	i.recentIndex++
	if i.recentIndex >= nameInternerRecentSize {
		i.recentIndex = 0
	}
}

func (i *nameInterner) internBytesHash(name []byte, prefixLen int, hash uint64) qnameSpan {
	if i.entries == nil {
		i.entries = make(map[uint64][]internEntry, 64)
	}
	if i.maxEntries < 0 {
		i.maxEntries = 0
	}

	if cached, ok := i.lookupRecent(name, hash); ok {
		i.stats.Hits++
		return cached
	}
	if bucket, ok := i.entries[hash]; ok {
		for _, entry := range bucket {
			if bytes.Equal(entry.name.Full.bytesUnsafe(), name) {
				i.stats.Hits++
				i.rememberRecent(entry.name, entry.hash)
				return entry.name
			}
		}
	}

	i.stats.Misses++
	if i.maxEntries > 0 && i.stats.Count >= i.maxEntries {
		buf := spanBuffer{data: append([]byte(nil), name...), stable: true}
		colon := -1
		if prefixLen > 0 && prefixLen < len(name) {
			colon = prefixLen
		}
		return makeQNameSpan(&buf, 0, len(name), colon)
	}

	start := len(i.buf.data)
	i.buf.data = append(i.buf.data, name...)
	end := len(i.buf.data)
	colon := -1
	if prefixLen > 0 && prefixLen < len(name) {
		colon = start + prefixLen
	}
	qname := makeQNameSpan(&i.buf, start, end, colon)
	entry := internEntry{hash: hash, name: qname}
	i.entries[hash] = append(i.entries[hash], entry)
	i.stats.Count++
	i.rememberRecent(entry.name, entry.hash)
	return qname
}

func newQNameSpan(buf *spanBuffer, start, end int) qnameSpan {
	full := makeSpan(buf, start, end)
	colon := bytes.IndexByte(full.bytesUnsafe(), ':')
	if colon < 0 {
		return qnameSpan{Full: full, Local: full}
	}
	return makeQNameSpan(buf, start, end, start+colon)
}

func makeQNameSpan(buf *spanBuffer, start, end, colon int) qnameSpan {
	full := makeSpan(buf, start, end)
	if colon < start || colon >= end {
		return qnameSpan{Full: full, Local: full}
	}
	prefix := makeSpan(buf, start, colon)
	local := makeSpan(buf, colon+1, end)
	return qnameSpan{Full: full, Prefix: prefix, Local: local, HasPrefix: true}
}

func hashBytes(data []byte) uint64 {
	return maphash.Bytes(hashSeed, data)
}
