package xsdxml

import "testing"

func TestReleaseDocumentTrimsLargeBuffers(t *testing.T) {
	doc := AcquireDocument()
	doc.nodes = make([]node, 0, maxPooledNodeEntries+1)
	doc.attrs = make([]Attr, 0, maxPooledAttrEntries+1)
	doc.children = make([]NodeID, 0, maxPooledChildEntries+1)
	doc.textSegments = make([]textSegment, 0, maxPooledTextSegmentEntries+1)
	doc.textScratch = make([]textScratchEntry, 0, maxPooledTextScratchEntries+1)
	doc.countsScratch = make([]int, 0, maxPooledCountEntries+1)
	ReleaseDocument(doc)

	reused := AcquireDocument()
	defer ReleaseDocument(reused)

	if cap(reused.nodes) > maxPooledNodeEntries {
		t.Fatalf("nodes cap = %d, want <= %d", cap(reused.nodes), maxPooledNodeEntries)
	}
	if cap(reused.attrs) > maxPooledAttrEntries {
		t.Fatalf("attrs cap = %d, want <= %d", cap(reused.attrs), maxPooledAttrEntries)
	}
	if cap(reused.children) > maxPooledChildEntries {
		t.Fatalf("children cap = %d, want <= %d", cap(reused.children), maxPooledChildEntries)
	}
	if cap(reused.textSegments) > maxPooledTextSegmentEntries {
		t.Fatalf("textSegments cap = %d, want <= %d", cap(reused.textSegments), maxPooledTextSegmentEntries)
	}
	if cap(reused.textScratch) > maxPooledTextScratchEntries {
		t.Fatalf("textScratch cap = %d, want <= %d", cap(reused.textScratch), maxPooledTextScratchEntries)
	}
	if cap(reused.countsScratch) > maxPooledCountEntries {
		t.Fatalf("countsScratch cap = %d, want <= %d", cap(reused.countsScratch), maxPooledCountEntries)
	}
}

func BenchmarkAcquireReleaseLargeDocument(b *testing.B) {
	for i := 0; i < b.N; i++ {
		doc := AcquireDocument()
		doc.nodes = make([]node, 0, maxPooledNodeEntries*2)
		doc.children = make([]NodeID, 0, maxPooledChildEntries*2)
		doc.attrs = make([]Attr, 0, maxPooledAttrEntries*2)
		ReleaseDocument(doc)
	}
}
