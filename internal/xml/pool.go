package xml

import "sync"

var documentPool = sync.Pool{
	New: func() any {
		return &Document{root: InvalidNode}
	},
}

// AcquireDocument returns a reusable XML document arena.
func AcquireDocument() *Document {
	doc := documentPool.Get().(*Document)
	doc.reset()
	return doc
}

// ReleaseDocument returns a document to the pool after resetting it.
func ReleaseDocument(doc *Document) {
	if doc == nil {
		return
	}
	doc.reset()
	documentPool.Put(doc)
}
