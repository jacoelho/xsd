package validate

import (
	"encoding/xml"
	"strconv"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlns"
)

func TestSessionResetDropsOversizedDocumentState(t *testing.T) {
	var s session
	s.doc.errors = make([]error, 1, maxRetainedSliceCap+1)
	s.doc.stack = make([]frame, 1, maxRetainedSliceCap+1)
	s.doc.ns = xmlns.NewStackWithCapacity(maxRetainedSliceCap+1, maxRetainedSliceCap+1)
	s.doc.text = make([]byte, 1, maxRetainedBufferCap+1)
	s.doc.path = make([]string, 1, maxRetainedSliceCap+1)
	s.doc.namePath = make([]runtime.RuntimeName, 1, maxRetainedSliceCap+1)
	s.doc.elementNames = make([]xml.Name, 1, maxRetainedSliceCap+1)
	s.doc.allBits = make([]uint64, 1, maxRetainedSliceCap+1)
	s.pathCache = make(map[pathCacheKey]string, maxRetainedMapLen+1)
	for i := range maxRetainedMapLen + 1 {
		key := strconv.Itoa(i)
		s.pathCache[pathCacheKey{Parent: key, Local: key}] = key
	}
	if err := recordValueForTest(&s.doc.identity, IdentityValue{IDs: "stale"}, s.startContext(1, 1)); err != nil {
		t.Fatalf("record identity state: %v", err)
	}
	if err := s.doc.schemaLocationHints.RecordAttribute(staleSchemaLocationHintName(), "urn:stale stale.xsd", s.startContext(1, 1)); err != nil {
		t.Fatalf("record schema-location hint: %v", err)
	}

	s.reset()

	if cap(s.doc.errors) != 0 ||
		cap(s.doc.stack) != 0 ||
		s.doc.ns.FrameCapacity() != 0 ||
		s.doc.ns.BindingCapacity() != 0 ||
		cap(s.doc.text) != 0 ||
		cap(s.doc.path) != 0 ||
		cap(s.doc.namePath) != 0 ||
		cap(s.doc.elementNames) != 0 ||
		cap(s.doc.allBits) != 0 {
		t.Fatalf("reset retained oversized state")
	}
	if err := s.doc.identity.CheckIDRefs(func(err error) error {
		t.Fatalf("identity state retained after reset: %v", err)
		return nil
	}); err != nil {
		t.Fatalf("check IDREFs after reset: %v", err)
	}
	if s.pathCache != nil {
		t.Fatalf("path cache retained after reset")
	}
	if s.doc.schemaLocationHints.Has("urn:stale") {
		t.Fatalf("schema location hint retained after reset")
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

func staleSchemaLocationHintName() xml.Name {
	return xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrSchemaLocation}
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
