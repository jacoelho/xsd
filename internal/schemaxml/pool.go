package schemaxml

import (
	"sync"
	"sync/atomic"
)

const (
	maxPooledNodeEntries        = 1 << 15
	maxPooledAttrEntries        = 1 << 15
	maxPooledChildEntries       = 1 << 16
	maxPooledTextSegmentEntries = 1 << 15
	maxPooledTextScratchEntries = 1 << 15
	maxPooledCountEntries       = 1 << 15
)

// DocumentPool stores reusable document arenas.
type DocumentPool struct {
	pool sync.Pool
	once sync.Once

	acquires atomic.Uint64
	releases atomic.Uint64
}

// DocumentPoolStats reports pool operation counts.
type DocumentPoolStats struct {
	Acquires uint64
	Releases uint64
}

// NewDocumentPool returns a reusable document arena pool.
func NewDocumentPool() *DocumentPool {
	p := &DocumentPool{}
	p.ensureInitialized()
	return p
}

func (p *DocumentPool) ensureInitialized() {
	if p == nil {
		return
	}
	p.once.Do(func() {
		p.pool.New = func() any {
			return &Document{root: InvalidNode}
		}
	})
}

// Acquire returns a reusable XML document arena.
// The caller owns the document until it is released back to the pool.
func (p *DocumentPool) Acquire() *Document {
	if p == nil {
		doc := &Document{root: InvalidNode}
		doc.reset()
		return doc
	}

	p.ensureInitialized()
	p.acquires.Add(1)
	raw := p.pool.Get()
	doc, ok := raw.(*Document)
	if !ok || doc == nil {
		doc = &Document{root: InvalidNode}
	}
	doc.reset()
	return doc
}

// Release returns a document to the pool after resetting it.
// After Release, the caller must not retain or use the document.
func (p *DocumentPool) Release(doc *Document) {
	if doc == nil {
		return
	}
	doc.reset()
	doc.trimForPool()
	if p == nil {
		return
	}
	p.releases.Add(1)
	p.pool.Put(doc)
}

// Stats returns acquire/release counters for the pool.
func (p *DocumentPool) Stats() DocumentPoolStats {
	if p == nil {
		return DocumentPoolStats{}
	}
	return DocumentPoolStats{
		Acquires: p.acquires.Load(),
		Releases: p.releases.Load(),
	}
}
