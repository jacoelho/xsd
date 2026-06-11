package xsd

import (
	"encoding/xml"
	"strconv"
	"testing"
)

func TestSessionResetDropsOversizedDocumentState(t *testing.T) {
	var s session
	s.doc.errors = make([]error, 1, maxRetainedSliceCap+1)
	s.doc.stack = make([]frame, 1, maxRetainedSliceCap+1)
	s.doc.ns.frames = make([]int, 1, maxRetainedSliceCap+1)
	s.doc.ns.bindings = make([]namespaceBinding, 1, maxRetainedSliceCap+1)
	s.doc.text = make([]byte, 1, maxRetainedBufferCap+1)
	s.doc.path = make([]string, 1, maxRetainedSliceCap+1)
	s.doc.namePath = make([]runtimeName, 1, maxRetainedSliceCap+1)
	s.doc.elementNames = make([]xml.Name, 1, maxRetainedSliceCap+1)
	s.doc.allBits = make([]uint64, 1, maxRetainedSliceCap+1)
	s.doc.idrefs = make([]identityRef, 1, maxRetainedSliceCap+1)
	s.doc.idScopes = make([]identityScope, 1, maxRetainedSliceCap+1)
	s.doc.idSelections = make([]identitySelection, 1, maxRetainedSliceCap+1)
	s.doc.identityFieldValues = make([]identityFieldValue, 1, maxRetainedSliceCap+1)
	s.doc.identityMatches = make([]identityFieldMatch, 1, maxRetainedSliceCap+1)
	s.doc.ids = make(map[string]string, maxRetainedMapLen+1)
	s.pathCache = make(map[pathCacheKey]string, maxRetainedMapLen+1)
	s.doc.schemaLocationNamespaces = make(map[string]bool, maxRetainedMapLen+1)
	for i := range maxRetainedMapLen + 1 {
		key := strconv.Itoa(i)
		s.doc.ids[key] = key
		s.pathCache[pathCacheKey{Parent: key, Local: key}] = key
		s.doc.schemaLocationNamespaces[key] = true
	}

	s.reset()

	if cap(s.doc.errors) != 0 ||
		cap(s.doc.stack) != 0 ||
		cap(s.doc.ns.frames) != 0 ||
		cap(s.doc.ns.bindings) != 0 ||
		cap(s.doc.text) != 0 ||
		cap(s.doc.path) != 0 ||
		cap(s.doc.namePath) != 0 ||
		cap(s.doc.elementNames) != 0 ||
		cap(s.doc.allBits) != 0 ||
		cap(s.doc.idrefs) != 0 ||
		cap(s.doc.idScopes) != 0 ||
		cap(s.doc.idSelections) != 0 ||
		cap(s.doc.identityFieldValues) != 0 ||
		cap(s.doc.identityMatches) != 0 {
		t.Fatalf("reset retained oversized state")
	}
	if s.doc.ids != nil {
		t.Fatalf("ids map retained after reset")
	}
	if s.pathCache != nil {
		t.Fatalf("path cache retained after reset")
	}
	if s.doc.schemaLocationNamespaces != nil {
		t.Fatalf("schema location namespace map retained after reset")
	}
}

func TestSessionResetClearsRetainedSliceCapacity(t *testing.T) {
	var s session
	s.doc.path = append(make([]string, 0, maxRetainedSliceCap), "stale")
	s.doc.path = s.doc.path[:0]
	s.doc.pathText = "stale"
	s.doc.pathTextDepth = 1

	s.reset()

	if s.doc.pathText != "" {
		t.Fatal("reset retained stale path text")
	}
	if s.doc.pathTextDepth != 0 {
		t.Fatal("reset retained stale path text depth")
	}
	if cap(s.doc.path) == 0 {
		t.Fatal("path capacity was not retained")
	}
	if s.doc.path[:cap(s.doc.path)][0] != "" {
		t.Fatal("reset retained stale path string")
	}
}

func TestSessionPathStringMaterializesLazily(t *testing.T) {
	var s session
	s.pushPath("root")
	s.pushPath("row")

	if s.doc.pathText != "" {
		t.Fatal("pushPath materialized path text")
	}
	if len(s.pathCache) != 0 {
		t.Fatal("pushPath populated path cache")
	}
	if got := s.pathString(); got != "/root/row" {
		t.Fatalf("pathString() = %q, want /root/row", got)
	}
	if s.doc.pathTextDepth != len(s.doc.path) {
		t.Fatalf("path text depth = %d, want %d", s.doc.pathTextDepth, len(s.doc.path))
	}
}

func TestSessionPopPathReturnsCachedParentPath(t *testing.T) {
	var s session
	s.pushPath("root")
	if got := s.pathString(); got != "/root" {
		t.Fatalf("pathString() = %q, want /root", got)
	}
	s.pushPath("child")
	if got := s.pathString(); got != "/root/child" {
		t.Fatalf("pathString() = %q, want /root/child", got)
	}

	s.popPath()

	if got := s.pathString(); got != "/root" {
		t.Fatalf("pathString() after pop = %q, want /root", got)
	}
	if s.doc.pathText != "/root" {
		t.Fatalf("pathText after pop = %q, want /root", s.doc.pathText)
	}
}
