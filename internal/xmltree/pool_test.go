package xmltree

import "testing"

func TestAcquireZeroValuePoolDoesNotPanic(t *testing.T) {
	t.Parallel()

	pool := &DocumentPool{}
	doc := pool.Acquire()
	if doc == nil {
		t.Fatal("Acquire() returned nil document")
	}

	pool.Release(doc)
	reused := pool.Acquire()
	if reused == nil {
		t.Fatal("Acquire() returned nil document after release")
	}
	pool.Release(reused)
}

func TestAcquireZeroValuePoolStatsAdvance(t *testing.T) {
	t.Parallel()

	pool := &DocumentPool{}
	before := pool.Stats()

	doc := pool.Acquire()
	pool.Release(doc)

	after := pool.Stats()
	if after.Acquires != before.Acquires+1 {
		t.Fatalf("Acquires = %d, want %d", after.Acquires, before.Acquires+1)
	}
	if after.Releases != before.Releases+1 {
		t.Fatalf("Releases = %d, want %d", after.Releases, before.Releases+1)
	}
}

func TestAcquireNilReceiverDoesNotPanic(t *testing.T) {
	t.Parallel()

	var pool *DocumentPool
	doc := pool.Acquire()
	if doc == nil {
		t.Fatal("Acquire() on nil receiver returned nil document")
	}
	pool.Release(doc)
}

func TestReleaseDocumentTrimsLargeBuffers(t *testing.T) {
	pool := NewDocumentPool()
	doc := pool.Acquire()
	doc.nodes = make([]node, 0, maxPooledNodeEntries+1)
	doc.attrs = make([]Attr, 0, maxPooledAttrEntries+1)
	doc.children = make([]NodeID, 0, maxPooledChildEntries+1)
	doc.textSegments = make([]textSegment, 0, maxPooledTextSegmentEntries+1)
	doc.textScratch = make([]textScratchEntry, 0, maxPooledTextScratchEntries+1)
	doc.countsScratch = make([]int, 0, maxPooledCountEntries+1)
	pool.Release(doc)

	reused := pool.Acquire()
	defer pool.Release(reused)

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
	pool := NewDocumentPool()
	for b.Loop() {
		doc := pool.Acquire()
		doc.nodes = make([]node, 0, maxPooledNodeEntries*2)
		doc.children = make([]NodeID, 0, maxPooledChildEntries*2)
		doc.attrs = make([]Attr, 0, maxPooledAttrEntries*2)
		pool.Release(doc)
	}
}
