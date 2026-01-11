package xmltext

import "bytes"

// InternStats reports QName interning activity.
type InternStats struct {
	Count  int
	Hits   int
	Misses int
}

type internEntry struct {
	hash uint64
	name QNameSpan
}

type nameInterner struct {
	entries    map[uint64][]internEntry
	buf        spanBuffer
	maxEntries int
	stats      InternStats
}

func newNameInterner(maxEntries int) *nameInterner {
	return &nameInterner{
		entries:    make(map[uint64][]internEntry, 64),
		maxEntries: maxEntries,
	}
}

func (i *nameInterner) setMax(maxEntries int) {
	i.maxEntries = maxEntries
}

func (i *nameInterner) intern(name []byte) QNameSpan {
	if len(name) == 0 {
		return QNameSpan{}
	}
	if i.entries == nil {
		i.entries = make(map[uint64][]internEntry, 64)
	}
	if i.maxEntries < 0 {
		i.maxEntries = 0
	}

	hash := hashBytes(name)
	if bucket, ok := i.entries[hash]; ok {
		for _, entry := range bucket {
			if bytes.Equal(entry.name.Full.bytes(), name) {
				i.stats.Hits++
				return entry.name
			}
		}
	}

	i.stats.Misses++
	if i.maxEntries > 0 && i.stats.Count >= i.maxEntries {
		return newQNameSpan(&spanBuffer{data: append([]byte(nil), name...)}, 0, len(name))
	}

	start := len(i.buf.data)
	i.buf.data = append(i.buf.data, name...)
	end := len(i.buf.data)
	qname := newQNameSpan(&i.buf, start, end)
	i.entries[hash] = append(i.entries[hash], internEntry{hash: hash, name: qname})
	i.stats.Count++
	return qname
}

func newQNameSpan(buf *spanBuffer, start, end int) QNameSpan {
	full := makeSpan(buf, start, end)
	colon := bytes.IndexByte(full.bytes(), ':')
	if colon < 0 {
		return QNameSpan{Full: full, Local: full}
	}
	colon += start
	prefix := makeSpan(buf, start, colon)
	local := makeSpan(buf, colon+1, end)
	return QNameSpan{Full: full, Prefix: prefix, Local: local, HasPrefix: true}
}

func hashBytes(data []byte) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	hash := uint64(offset)
	for _, b := range data {
		hash ^= uint64(b)
		hash *= prime
	}
	return hash
}
