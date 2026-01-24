package xsdxml

import "sync"

var documentPool = sync.Pool{
	New: func() any {
		return &Document{root: InvalidNode}
	},
}

// AcquireDocument returns a reusable XML document arena.
// The caller owns the document until it is released back to the pool.
func AcquireDocument() *Document {
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
	documentPool.Put(doc)
}
