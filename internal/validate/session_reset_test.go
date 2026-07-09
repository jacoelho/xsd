package validate

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
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

func TestSessionResetClearsActiveDocumentReferences(t *testing.T) {
	var s session
	s.doc.elements = make([]xmlDocumentElement, 0, maxRetainedSliceCap)
	s.doc.CommitStart(xml.Name{Local: "stale"}, "stale", false)
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
	s.doc.CommitStart(xml.Name{Local: "root"}, "root", false)
	s.doc.CommitStart(xml.Name{Local: "row"}, "row", false)

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
	s.doc.CommitStart(xml.Name{Local: "root"}, "root", false)
	if got := s.doc.PathString(); got != "/root" {
		t.Fatalf("pathString() = %q, want /root", got)
	}
	s.doc.CommitStart(xml.Name{Local: "child"}, "child", false)
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
	s.doc.CommitStart(xml.Name{Local: "root"}, "root", false)

	err := s.chars(1, 1, nil, false)
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
}

func TestSessionLifecycleZeroesReleasedReferences(t *testing.T) {
	rt, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType">
    <xs:key name="ids"><xs:selector xpath=".//never"/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	const depth = 128
	doc := nestedIdentityDocument(depth, true)

	t.Run("completed document", func(t *testing.T) {
		session, err := NewSession(rt, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			t.Fatal(err)
		}
		assertReleasedReferencesZero(t, &session.session)
	})

	t.Run("aborted document reset", func(t *testing.T) {
		session, err := NewSession(rt, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Validate(strings.NewReader(nestedIdentityDocument(depth, false))); err == nil {
			t.Fatal("Validate() succeeded for unclosed document")
		}
		session.Reset()
		assertReleasedReferencesZero(t, &session.session)
	})
}

func nestedIdentityDocument(depth int, closeElements bool) string {
	var b strings.Builder
	b.WriteString("<root>")
	for i := range depth {
		fmt.Fprintf(&b, `<a id="%d">`, i)
	}
	if closeElements {
		for range depth {
			b.WriteString("</a>")
		}
		b.WriteString("</root>")
	}
	return b.String()
}

func assertReleasedReferencesZero(t *testing.T, s *session) {
	t.Helper()
	if len(s.doc.elements) != 0 || len(s.doc.stack) != 0 || len(s.doc.namePath) != 0 {
		t.Fatalf("active document state remains: elements=%d stack=%d names=%d", len(s.doc.elements), len(s.doc.stack), len(s.doc.namePath))
	}
	for i, element := range s.doc.elements[:cap(s.doc.elements)] {
		if element != (xmlDocumentElement{}) {
			t.Fatalf("element tail %d retains references: %+v", i, element)
		}
	}
	for i, name := range s.doc.namePath[:cap(s.doc.namePath)] {
		if name != (runtime.RuntimeName{}) {
			t.Fatalf("name path tail %d retains references: %+v", i, name)
		}
	}
	identity := &s.doc.identity
	for i, scope := range identity.scopes[:cap(identity.scopes)] {
		if scope.tables != nil || scope.constraints != nil || scope.refs != nil {
			t.Fatalf("identity scope tail %d retains references: %+v", i, scope)
		}
	}
	for i, selection := range identity.selections[:cap(identity.selections)] {
		if selection.path != "" {
			t.Fatalf("identity selection tail %d retains path %q", i, selection.path)
		}
	}
	for i, field := range identity.fieldValues[:cap(identity.fieldValues)] {
		if field.value != "" {
			t.Fatalf("identity field tail %d retains value %q", i, field.value)
		}
	}
}
