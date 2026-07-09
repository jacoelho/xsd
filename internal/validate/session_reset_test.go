package validate

import (
	"encoding/xml"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSessionResetDropsOversizedDocumentState(t *testing.T) {
	var s session
	s.doc.errors = make([]error, 1, maxRetainedSliceCap+1)
	s.doc.stack = make([]frame, 1, maxRetainedSliceCap+1)
	s.doc.ns = xmlns.NewStackWithCapacity(maxRetainedSliceCap+1, maxRetainedSliceCap+1)
	s.doc.elements = make([]xmlDocumentElement, 1, maxRetainedSliceCap+1)
	s.doc.text = make([]byte, 1, maxRetainedBufferCap+1)
	s.doc.namePath = make([]runtime.RuntimeName, 1, maxRetainedSliceCap+1)
	s.doc.allBits = make([]uint64, 1, maxRetainedSliceCap+1)
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
		cap(s.doc.elements) != 0 ||
		cap(s.doc.text) != 0 ||
		cap(s.doc.namePath) != 0 ||
		cap(s.doc.allBits) != 0 {
		t.Fatalf("reset retained oversized state")
	}
	if err := s.doc.identity.CheckIDRefs(func(err error) error {
		t.Fatalf("identity state retained after reset: %v", err)
		return nil
	}); err != nil {
		t.Fatalf("check IDREFs after reset: %v", err)
	}
	if s.doc.schemaLocationHints.Has("urn:stale") {
		t.Fatalf("schema location hint retained after reset")
	}
}

func TestSessionResetClearsRetainedSliceCapacity(t *testing.T) {
	var s session
	s.doc.elements = append(make([]xmlDocumentElement, 0, maxRetainedSliceCap), xmlDocumentElement{pathLabel: "stale"})
	s.doc.elements = s.doc.elements[:0]
	s.doc.pathText = "stale"
	s.doc.pathTextDepth = 1

	s.reset()

	if s.doc.pathText != "" {
		t.Fatal("reset retained stale path text")
	}
	if s.doc.pathTextDepth != 0 {
		t.Fatal("reset retained stale path text depth")
	}
	if cap(s.doc.elements) == 0 {
		t.Fatal("path capacity was not retained")
	}
	if s.doc.elements[:cap(s.doc.elements)][0] != (xmlDocumentElement{}) {
		t.Fatal("reset retained stale path string")
	}
}

func staleSchemaLocationHintName() xml.Name {
	return xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrSchemaLocation}
}

func TestSessionPathStringMaterializesLazily(t *testing.T) {
	var s session
	s.doc.CommitStart(xml.Name{Local: "root"}, "root", "root")
	s.doc.CommitStart(xml.Name{Local: "row"}, "row", "row")

	if s.doc.pathText != "" {
		t.Fatal("pushPath materialized path text")
	}
	if got := s.doc.PathString(); got != "/root/row" {
		t.Fatalf("pathString() = %q, want /root/row", got)
	}
	if s.doc.pathTextDepth != s.doc.Depth() {
		t.Fatalf("path text depth = %d, want %d", s.doc.pathTextDepth, s.doc.Depth())
	}
}

func TestSessionPopPathReturnsCachedParentPath(t *testing.T) {
	var s session
	s.doc.CommitStart(xml.Name{Local: "root"}, "root", "root")
	if got := s.doc.PathString(); got != "/root" {
		t.Fatalf("pathString() = %q, want /root", got)
	}
	s.doc.CommitStart(xml.Name{Local: "child"}, "child", "child")
	if got := s.doc.PathString(); got != "/root/child" {
		t.Fatalf("pathString() = %q, want /root/child", got)
	}

	if err := s.doc.CommitEnd(); err != nil {
		t.Fatal(err)
	}

	if got := s.doc.PathString(); got != "/root" {
		t.Fatalf("pathString() after pop = %q, want /root", got)
	}
	if s.doc.pathText != "/root" {
		t.Fatalf("pathText after pop = %q, want /root", s.doc.pathText)
	}
}

func TestSessionRejectsDocumentSchemaDepthDivergence(t *testing.T) {
	var s session
	s.doc.CommitStart(xml.Name{Local: "root"}, "root", "root")

	err := s.chars(1, 1, nil, false)
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
}
