package validate

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
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

func TestSessionIdentityLimitsAreNotRecoverable(t *testing.T) {
	valueSchema, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:IDREFS"/>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		opts Options
		doc  string
		msg  string
	}{
		{
			name: "entry",
			opts: Options{MaxIdentityEntries: 1},
			doc:  "<root>a b c</root>",
			msg:  "identity entry limit exceeded",
		},
		{
			name: "tuple bytes",
			opts: Options{MaxIdentityTupleBytes: 3},
			doc:  "<root>longer</root>",
			msg:  "identity tuple byte limit exceeded",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSession, sessionErr := newSessionForTest(valueSchema, tt.opts)
			if sessionErr != nil {
				t.Fatal(sessionErr)
			}
			assertSingleIdentityLimit(t, testSession.Validate(strings.NewReader(tt.doc)), tt.msg)
		})
	}

	scopeSchema, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element ref="root" minOccurs="0"/></xs:sequence>
    </xs:complexType>
    <xs:key name="unused">
      <xs:selector xpath="never"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	session, err := newSessionForTest(scopeSchema, Options{MaxIdentityScopes: 1})
	if err != nil {
		t.Fatal(err)
	}
	assertSingleIdentityLimit(t, session.Validate(strings.NewReader("<root><root/></root>")), "identity scope limit exceeded")
}

func assertSingleIdentityLimit(t *testing.T, err error, message string) {
	t.Helper()
	requireCode(t, err, xsderrors.CodeValidationLimit)
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("Validate() error = %v, want %q", err, message)
	}
	if multiple, ok := errors.AsType[xsderrors.Errors](err); ok {
		t.Fatalf("Validate() returned recoverable error collection: %v", multiple)
	}
}

func TestSessionDoesNotResetCallerBufferedReader(t *testing.T) {
	rt, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	session, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}

	var source bytes.Buffer
	source.WriteString("<root/>")
	callerReader := bufio.NewReaderSize(&source, 128*1024)
	if err = session.Validate(callerReader); err != nil {
		t.Fatal(err)
	}
	if err = session.Validate(strings.NewReader("<root/>")); err != nil {
		t.Fatal(err)
	}

	source.WriteString("sentinel")
	got, err := callerReader.ReadString('l')
	if err != nil {
		t.Fatalf("caller reader was reset by session reuse: %v", err)
	}
	if got != "sentinel" {
		t.Fatalf("caller reader returned %q, want sentinel", got)
	}
}

func TestSessionDetachesReaderAfterPreflightFailure(t *testing.T) {
	rt, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	session, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	invalid := string([]byte{0xFE, 0xFF}) + strings.Repeat("x", 1024)
	if err := session.Validate(strings.NewReader(invalid)); err == nil {
		t.Fatal("Validate() accepted UTF-16 input")
	}
	if err := session.Validate(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("Validate() after preflight failure: %v", err)
	}
}

func TestReusableSessionClearsDocumentStateBeforeReturning(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:simpleContent><xs:extension base="xs:string">
      <xs:attribute name="id" type="xs:ID" use="required"/>
    </xs:extension></xs:simpleContent></xs:complexType>
    <xs:key name="ids"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`)
	const start = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:payload" id="item" xsi:schemaLocation="urn:hint hint.xsd">payload`

	t.Run("success", func(t *testing.T) {
		s, err := newSessionForTest(rt, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(strings.NewReader(start + `</root>`)); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		assertReusableSessionReset(t, s)
		if cap(s.session.doc.text) == 0 || cap(s.session.doc.text) > maxRetainedBufferCap {
			t.Fatalf("retained text capacity = %d, want 1..%d", cap(s.session.doc.text), maxRetainedBufferCap)
		}
	})

	for _, test := range []struct {
		name string
		doc  string
	}{
		{name: "unclosed", doc: start},
		{name: "mismatched end", doc: start + `</other>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, err := newSessionForTest(rt, Options{})
			if err != nil {
				t.Fatal(err)
			}
			validationErr := s.Validate(strings.NewReader(test.doc))
			requireCode(t, validationErr, xsderrors.CodeValidationXML)
			assertReusableSessionReset(t, s)
		})
	}
}

func TestReusableSessionCleanupPreservesReturnedAggregateErrors(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="value" type="xs:int" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	s, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}

	validationErr := s.Validate(strings.NewReader(`<root><value>first</value><value>second</value></root>`))
	errs, ok := errors.AsType[xsderrors.Errors](validationErr)
	if !ok || len(errs) != 2 {
		t.Fatalf("Validate() error = %v, want two returned errors", validationErr)
	}
	for i, err := range errs {
		xerr, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || xerr.Code != xsderrors.CodeValidationFacet {
			t.Fatalf("Validate() error %d = %v, want validation facet error", i, err)
		}
	}
	want := validationErr.Error()
	assertReusableSessionReset(t, s)

	if err := s.Validate(strings.NewReader(`<root><value>1</value></root>`)); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}
	assertReusableSessionReset(t, s)
	if got := validationErr.Error(); got != want {
		t.Fatalf("returned aggregate changed after cleanup and reuse:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestSessionResetDropsOversizedDocumentState(t *testing.T) {
	var s session
	s.doc.errors = make([]error, 1, maxRetainedSliceCap+1)
	s.doc.ns = xmlns.NewStackWithCapacity(maxRetainedSliceCap+1, maxRetainedSliceCap+1)
	s.doc.elements = make([]xmlDocumentElement[frame], 1, maxRetainedSliceCap+1)
	s.doc.text = make([]byte, 1, maxRetainedBufferCap+1)
	s.doc.namePath = make([]runtime.RuntimeName, 1, maxRetainedSliceCap+1)
	s.doc.allBits = make([]uint64, 1, maxRetainedSliceCap+1)
	if err := recordValueForTest(&s.doc.identity, IdentityValue{IDs: "stale"}, s.startContext(1, 1)); err != nil {
		t.Fatalf("record identity state: %v", err)
	}
	if err := s.doc.schemaLocationHints.RecordAttribute(staleSchemaLocationHintName(), "urn:stale stale.xsd", testSchemaLocationHintLimits, s.startContext(1, 1)); err != nil {
		t.Fatalf("record schema-location hint: %v", err)
	}

	s.reset()

	if cap(s.doc.errors) != 0 ||
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
	s.doc.elements = make([]xmlDocumentElement[frame], 0, maxRetainedSliceCap)
	s.doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "stale"}}, false, frame{})
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
	if s.doc.elements[:cap(s.doc.elements)][0] != (xmlDocumentElement[frame]{}) {
		t.Fatal("reset retained stale path string")
	}
}

func staleSchemaLocationHintName() xml.Name {
	return xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrSchemaLocation}
}

func TestSessionPathStringMaterializesLazily(t *testing.T) {
	var s session
	s.doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, false, frame{})
	s.doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "row"}}, false, frame{})

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
	s.doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, false, frame{})
	if got := s.doc.PathString(); got != "/root" {
		t.Fatalf("pathString() = %q, want /root", got)
	}
	s.doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "child"}}, false, frame{})
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
		session, err := newSessionForTest(rt, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			t.Fatal(err)
		}
		assertReusableSessionReset(t, session)
	})

	t.Run("aborted document", func(t *testing.T) {
		session, err := newSessionForTest(rt, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Validate(strings.NewReader(nestedIdentityDocument(depth, false))); err == nil {
			t.Fatal("Validate() succeeded for unclosed document")
		}
		assertReusableSessionReset(t, session)
		if err := session.Validate(strings.NewReader(`<root/>`)); err != nil {
			t.Fatalf("Validate() after aborted document: %v", err)
		}
		assertReusableSessionReset(t, session)
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

func assertReusableSessionReset(t *testing.T, reusable *Session) {
	t.Helper()
	if reusable.inUse.Load() {
		t.Fatal("session remained in use after Validate returned")
	}
	assertSessionDocumentStateReset(t, &reusable.session)
}

func assertSessionDocumentStateReset(t *testing.T, s *session) {
	t.Helper()
	if s.doc.seenRoot || s.doc.pathText != "" || s.doc.pathTextDepth != 0 || s.doc.syntaxOnly {
		t.Fatalf("document scalars remain after reset: %+v", s.doc)
	}
	if len(s.doc.elements) != 0 || len(s.doc.namePath) != 0 {
		t.Fatalf("active document state remains: elements=%d names=%d", len(s.doc.elements), len(s.doc.namePath))
	}
	if len(s.doc.errors) != 0 || len(s.doc.text) != 0 || len(s.doc.allBits) != 0 {
		t.Fatalf("document buffers remain: errors=%d text=%d allBits=%d", len(s.doc.errors), len(s.doc.text), len(s.doc.allBits))
	}
	if _, ok := s.doc.LookupNamespace("xsi"); ok {
		t.Fatal("namespace bindings remain after reset")
	}
	for i, err := range s.doc.errors[:cap(s.doc.errors)] {
		if err != nil {
			t.Fatalf("error tail %d retains %v", i, err)
		}
	}
	for i, element := range s.doc.elements[:cap(s.doc.elements)] {
		if element != (xmlDocumentElement[frame]{}) {
			t.Fatalf("element tail %d retains references: %+v", i, element)
		}
	}
	for i, name := range s.doc.namePath[:cap(s.doc.namePath)] {
		if name != (runtime.RuntimeName{}) {
			t.Fatalf("name path tail %d retains references: %+v", i, name)
		}
	}
	identity := &s.doc.identity
	if len(identity.ids) != 0 || len(identity.idrefs) != 0 || len(identity.scopes) != 0 ||
		len(identity.selections) != 0 || len(identity.fieldValues) != 0 || len(identity.matches) != 0 ||
		identity.entries != 0 || identity.nextNodeID != 0 {
		t.Fatalf("identity state remains after reset: %+v", *identity)
	}
	for i, ref := range identity.idrefs[:cap(identity.idrefs)] {
		if ref != (identityRef{}) {
			t.Fatalf("identity reference tail %d retains references: %+v", i, ref)
		}
	}
	for i, scope := range identity.scopes[:cap(identity.scopes)] {
		if scope.tables != nil || scope.constraints.Len() != 0 || scope.refs != nil || scope.depth != 0 || scope.invalid {
			t.Fatalf("identity scope tail %d retains references: %+v", i, scope)
		}
	}
	for i, selection := range identity.selections[:cap(identity.selections)] {
		if selection != (identitySelection{}) {
			t.Fatalf("identity selection tail %d retains state: %+v", i, selection)
		}
	}
	for i, field := range identity.fieldValues[:cap(identity.fieldValues)] {
		if field != (identityFieldValue{}) {
			t.Fatalf("identity field tail %d retains state: %+v", i, field)
		}
	}
	if len(s.doc.schemaLocationHints.namespaces) != 0 || s.doc.schemaLocationHints.namespaceBytes != 0 {
		t.Fatalf("schema-location hints remain after reset: %+v", s.doc.schemaLocationHints)
	}
}
