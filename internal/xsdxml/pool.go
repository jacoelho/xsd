package xsdxml

import "sync"

const (
	maxPooledNodeEntries        = 1 << 15
	maxPooledAttrEntries        = 1 << 15
	maxPooledChildEntries       = 1 << 16
	maxPooledTextSegmentEntries = 1 << 15
	maxPooledTextScratchEntries = 1 << 15
	maxPooledCountEntries       = 1 << 15
)

var documentPool = sync.Pool{
	New: func() any {
		return &Document{root: InvalidNode}
	},
}

// AcquireDocument returns a reusable XML document arena.
// The caller owns the document until it is released back to the pool.
func AcquireDocument() *Document {
	// documentPool stores only *Document values by construction.
	doc := documentPool.Get().(*Document)
	doc.reset()
	return doc
}

// ReleaseDocument returns a document to the pool after resetting it.
// After ReleaseDocument, the caller must not retain or use the document.
func ReleaseDocument(doc *Document) {
	if doc == nil {
		return
	}
	doc.reset()
	doc.trimForPool()
	documentPool.Put(doc)
}
