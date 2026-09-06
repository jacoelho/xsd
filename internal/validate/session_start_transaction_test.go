package validate

import (
	"encoding/xml"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSessionStartRollsBackRootOnFatalAssessment(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	var s session
	if err := initializeSession(&s, rt, Options{}); err != nil {
		t.Fatal(err)
	}
	err := startForTest(t, &s, 1, 1, testXMLStart(
		xml.Name{Local: "missing"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "xsi"}, vocab.XSINamespaceURI),
		testXMLAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrNoNamespaceSchemaLocation}, "missing.xsd"),
	))
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
	if s.doc.Depth() != 0 || s.doc.schemaLocationHints.Has("") {
		t.Fatalf("failed root start changed document state: depth=%d hints=%+v", s.doc.Depth(), s.doc.schemaLocationHints)
	}
	if len(s.doc.identity.path) != 0 || len(s.doc.identity.elements) != 0 {
		t.Fatal("failed root start changed identity state")
	}
	if _, ok := s.reader.Lookup("xsi"); ok {
		t.Fatal("failed root start retained namespace bindings")
	}
}

func TestSessionStartRollsBackParentContentOnFatalChildAssessment(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		<xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child" type="xs:string"/></xs:sequence></xs:complexType></xs:element>
	</xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	parent, ok := s.doc.Current()
	if !ok {
		t.Fatal("root start did not create a frame")
	}
	parentBefore := *parent
	err := startForTest(t, &s, 2, 1, xmlstream.OwnedStartElement(
		xml.Name{Local: "child"},
		xmlstream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "xsi"}, vocab.XSINamespaceURI),
		xmlstream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:missing"),
		xmlstream.OwnedAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrSchemaLocation}, "urn:missing missing.xsd"),
		xmlstream.OwnedAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrType}, "p:Missing"),
	))
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
	parent, ok = s.doc.Current()
	if !ok || s.doc.Depth() != 1 || *parent != parentBefore {
		t.Fatalf("failed child start changed parent: depth=%d frame=%+v want=%+v", s.doc.Depth(), parent, parentBefore)
	}
	if s.doc.schemaLocationHints.Has("urn:missing") {
		t.Fatal("failed child start retained schema-location hint")
	}
	if _, ok := s.reader.Lookup("p"); ok {
		t.Fatal("failed child start retained namespace bindings")
	}
}

func TestSessionStartRollsBackAllModelBitsOnFatalChildAssessment(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		<xs:element name="root"><xs:complexType><xs:all><xs:element name="child" type="xs:string"/></xs:all></xs:complexType></xs:element>
	</xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	allBitsBefore := slices.Clone(s.doc.allBits)
	err := startForTest(t, &s, 2, 1, xmlstream.OwnedStartElement(
		xml.Name{Local: "child"},
		xmlstream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "xsi"}, vocab.XSINamespaceURI),
		xmlstream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:missing"),
		xmlstream.OwnedAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrSchemaLocation}, "urn:missing missing.xsd"),
		xmlstream.OwnedAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrType}, "p:Missing"),
	))
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
	if !slices.Equal(s.doc.allBits, allBitsBefore) {
		t.Fatalf("failed child start changed xs:all bits: got %v want %v", s.doc.allBits, allBitsBefore)
	}
	if err := startForTest(t, &s, 2, 1, testXMLStart(xml.Name{Local: "child"})); err != nil {
		t.Fatalf("retry child start error = %v", err)
	}
	child, ok := s.doc.Current()
	if !ok || child.Mode != elementAssessed {
		t.Fatalf("retry child frame = %+v, want assessed", child)
	}
}

func TestSessionStartRollsBackStagedContentAfterFatalTypeAssessment(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="child" type="xs:string"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 1, 1, testXMLStart(
		xml.Name{Local: "root"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "xsi"}, vocab.XSINamespaceURI),
		testXMLAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrSchemaLocation}, "urn:missing missing.xsd"),
	)); err != nil {
		t.Fatal(err)
	}
	parent, ok := s.doc.Current()
	if !ok {
		t.Fatal("root start did not create a frame")
	}
	parentBefore := *parent
	identityPathLen := len(s.doc.identity.path)
	identityElementLen := len(s.doc.identity.elements)

	err := startForTest(t, &s, 2, 1, testXMLStart(
		xml.Name{Local: "child"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:missing"),
		testXMLAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrType}, "p:Missing"),
	))
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
	parent, ok = s.doc.Current()
	if !ok || s.doc.Depth() != 1 || *parent != parentBefore {
		t.Fatalf("failed staged child changed parent: depth=%d frame=%+v want=%+v", s.doc.Depth(), parent, parentBefore)
	}
	if !s.doc.schemaLocationHints.Has("urn:missing") {
		t.Fatal("failed child removed an inherited schema-location hint")
	}
	if _, ok := s.reader.Lookup("p"); ok {
		t.Fatal("failed child retained namespace bindings")
	}
	if len(s.doc.identity.path) != identityPathLen || len(s.doc.identity.elements) != identityElementLen ||
		s.doc.identity.startJournal.active {
		t.Fatal("failed staged child retained identity state")
	}
}

func TestSessionStartStagesParentInvalidationUntilCommit(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="child" type="xs:string"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	checkpoint := s.doc.startCheckpoint()
	_, err := nextStartForTest(t, &s, testXMLStart(xml.Name{Local: "unexpected"}))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := s.doc.PrepareStart(&s.reader, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := s.beginStartTransaction(checkpoint, prepared.handle)
	if err != nil {
		t.Fatal(err)
	}
	transaction.stageContent(acceptedChild{invalidatesParent: true})
	if abortErr := transaction.abort(); abortErr != nil {
		t.Fatal(abortErr)
	}
	parent, ok := s.doc.Current()
	if !ok || parent.AssessmentInvalid {
		t.Fatalf("aborted child invalidated parent: %+v", parent)
	}

	if err := startForTest(t, &s, 2, 1, testXMLStart(xml.Name{Local: "unexpected"})); err != nil {
		t.Fatal(err)
	}
	if parent := &s.doc.elements[0].payload; !parent.AssessmentInvalid {
		t.Fatal("committed recovery child did not invalidate parent assessment")
	}
}

func TestSessionStartTransactionReusesStorage(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{}); err != nil {
		t.Fatal(err)
	}
	end := xmlstream.EndElement{Name: xml.Name{Local: "root"}}
	run := func() {
		if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
			panic(err)
		}
		if err := endForTest(t, &s, 1, 8, end); err != nil {
			panic(err)
		}
		s.reset()
		sessionXMLState(&s).ready = false
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("start/end transaction allocations = %v, want 0 after warmup", allocs)
	}
}

func TestSessionStartRollsBackIdentityAfterXMLCommit(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="child">
        <xs:complexType>
          <xs:attribute name="a" type="xs:ID"/>
          <xs:attribute name="b" type="xs:IDREF"/>
        </xs:complexType>
      </xs:element>
    </xs:sequence></xs:complexType>
    <xs:key name="byA"><xs:selector xpath="child"/><xs:field xpath="@a"/></xs:key>
  </xs:element>
</xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{MaxIdentityEntries: 1}); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	parent, ok := s.doc.Current()
	if !ok {
		t.Fatal("root start did not create a frame")
	}
	parentBefore := *parent
	allBitsBefore := slices.Clone(s.doc.allBits)

	err := startForTest(t, &s, 2, 1, testXMLStart(
		xml.Name{Local: "child"},
		testXMLAttr(xml.Name{Local: "a"}, "one"),
		testXMLAttr(xml.Name{Local: "b"}, "two"),
	))
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)

	parent, ok = s.doc.Current()
	if !ok || s.doc.Depth() != 1 || *parent != parentBefore {
		t.Fatalf("failed child start changed parent: depth=%d frame=%+v want=%+v", s.doc.Depth(), parent, parentBefore)
	}
	if !slices.Equal(s.doc.allBits, allBitsBefore) {
		t.Fatalf("failed child start changed content bits: got %v want %v", s.doc.allBits, allBitsBefore)
	}
	identity := &s.doc.identity
	if len(identity.path) != 1 || len(identity.elements) != 1 || len(identity.scopes) != 1 ||
		len(identity.selections) != 0 || len(identity.fieldValues) != 0 || len(identity.idrefs) != 0 ||
		identity.entries != 0 || identity.nextNodeID != 0 || identity.targetPhase != identityTargetInactive ||
		identity.startJournal.active {
		t.Fatalf("failed child start retained identity state: %+v", identity.identityState)
	}
	if _, exists := identity.ids["one"]; exists {
		t.Fatal("failed child start retained ID")
	}
	if len(s.doc.retainedPaths.nodes) != 0 || s.doc.elements[0].pathRef != (documentPathRef{}) {
		t.Fatalf("failed child start retained diagnostic paths: nodes=%d root=%+v", len(s.doc.retainedPaths.nodes), s.doc.elements[0].pathRef)
	}
}

func TestSessionStartRollsBackPendingIdentityFieldLimit(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A"><xs:sequence><xs:element ref="a" minOccurs="0"/></xs:sequence></xs:complexType>
  <xs:element name="a" type="A"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="a" minOccurs="0"/></xs:sequence></xs:complexType>
    <xs:unique name="values"><xs:selector xpath=".//a"/><xs:field xpath="@left"/><xs:field xpath="@right"/></xs:unique>
  </xs:element>
</xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{MaxIdentityEntries: 3}); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 2, 1, testXMLStart(xml.Name{Local: "a"})); err != nil {
		t.Fatal(err)
	}
	parent, ok := s.doc.Current()
	if !ok {
		t.Fatal("selected parent start did not create a frame")
	}
	parentBefore := *parent
	allBitsBefore := slices.Clone(s.doc.allBits)

	err := startForTest(t, &s, 3, 1, testXMLStart(xml.Name{Local: "a"}))
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	parent, ok = s.doc.Current()
	if !ok || s.doc.Depth() != 2 || *parent != parentBefore {
		t.Fatalf("failed nested start changed parent: depth=%d frame=%+v want=%+v", s.doc.Depth(), parent, parentBefore)
	}
	if !slices.Equal(s.doc.allBits, allBitsBefore) {
		t.Fatalf("failed nested start changed content bits: got %v want %v", s.doc.allBits, allBitsBefore)
	}
	identity := &s.doc.identity
	if len(identity.path) != 2 || len(identity.elements) != 2 || len(identity.scopes) != 1 ||
		len(identity.selections) != 1 || len(identity.fieldValues) != 2 || identity.nextNodeID != 1 ||
		identity.targetPhase != identityTargetInactive || identity.startJournal.active {
		t.Fatalf("failed nested start retained identity state: %+v", identity.identityState)
	}

	identity.limits.Entries = 4
	if err := startForTest(t, &s, 3, 1, testXMLStart(xml.Name{Local: "a"})); err != nil {
		t.Fatalf("retry nested start error = %v", err)
	}
}

func TestSessionIdentitySelectionsKeepDocumentPathsLazy(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A"><xs:sequence><xs:element ref="a" minOccurs="0"/></xs:sequence></xs:complexType>
  <xs:element name="a" type="A"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="a" minOccurs="0"/></xs:sequence></xs:complexType>
    <xs:unique name="values"><xs:selector xpath=".//a"/><xs:field xpath="@missing"/></xs:unique>
  </xs:element>
</xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 2, 1, testXMLStart(xml.Name{Local: "a"})); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 3, 1, testXMLStart(xml.Name{Local: "a"})); err != nil {
		t.Fatal(err)
	}
	if s.doc.pathText != "" {
		t.Fatalf("selector starts materialized document path %q", s.doc.pathText)
	}
	if len(s.doc.retainedPaths.nodes) != 0 {
		t.Fatalf("selector starts retained %d document path nodes", len(s.doc.retainedPaths.nodes))
	}

	if err := endForTest(t, &s, 3, 1, xmlstream.EndElement{Name: xml.Name{Local: "a"}}); err != nil {
		t.Fatal(err)
	}
	if s.doc.pathText != "" {
		t.Fatalf("absent unique field materialized document path %q", s.doc.pathText)
	}
}

func TestSessionStartRollsBackCompositeStateAfterIdentityFailure(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:all>
      <xs:element name="child">
        <xs:complexType>
          <xs:attribute name="a" type="xs:ID"/>
          <xs:attribute name="b" type="xs:IDREF"/>
        </xs:complexType>
      </xs:element>
    </xs:all></xs:complexType>
    <xs:key name="byA"><xs:selector xpath="child"/><xs:field xpath="@a"/></xs:key>
  </xs:element>
</xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{MaxIdentityEntries: 1}); err != nil {
		t.Fatal(err)
	}
	if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	parent, ok := s.doc.Current()
	if !ok {
		t.Fatal("root start did not create a frame")
	}
	parentBefore := *parent
	bitsBefore := slices.Clone(s.doc.allBits)
	errorsBefore := len(s.doc.errors)

	err := startForTest(t, &s, 2, 1, testXMLStart(
		xml.Name{Local: "child"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "xsi"}, vocab.XSINamespaceURI),
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:admitted"),
		testXMLAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrSchemaLocation}, "urn:hint hint.xsd"),
		testXMLAttr(xml.Name{Local: "unexpected"}, "recoverable"),
		testXMLAttr(xml.Name{Local: "a"}, "one"),
		testXMLAttr(xml.Name{Local: "b"}, "two"),
	))
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)

	parent, ok = s.doc.Current()
	if !ok || s.doc.Depth() != 1 || *parent != parentBefore {
		t.Fatalf("failed child start changed parent: depth=%d frame=%+v want=%+v", s.doc.Depth(), parent, parentBefore)
	}
	if !slices.Equal(s.doc.allBits, bitsBefore) {
		t.Fatalf("failed child start changed xs:all bits: got %v want %v", s.doc.allBits, bitsBefore)
	}
	if len(s.doc.errors) != errorsBefore || s.doc.syntaxOnly {
		t.Fatalf("failed child retained recovery state: errors=%d syntaxOnly=%v", len(s.doc.errors), s.doc.syntaxOnly)
	}
	if s.doc.schemaLocationHints.Has("urn:hint") {
		t.Fatal("failed child retained schema-location hint")
	}
	if _, ok := s.reader.Lookup("p"); ok {
		t.Fatal("failed child retained namespace admission")
	}
	identity := &s.doc.identity
	if len(identity.path) != 1 || len(identity.elements) != 1 || len(identity.scopes) != 1 ||
		len(identity.selections) != 0 || len(identity.fieldValues) != 0 || len(identity.idrefs) != 0 ||
		identity.entries != 0 || identity.nextNodeID != 0 || identity.targetPhase != identityTargetInactive ||
		identity.startJournal.active {
		t.Fatalf("failed child start retained identity state: %+v", identity.identityState)
	}
	if _, exists := identity.ids["one"]; exists {
		t.Fatal("failed child start retained ID")
	}

	identity.limits.Entries = 4
	if err := startForTest(t, &s, 3, 1, testXMLStart(
		xml.Name{Local: "child"},
		testXMLAttr(xml.Name{Local: "a"}, "one"),
		testXMLAttr(xml.Name{Local: "b"}, "one"),
	)); err != nil {
		t.Fatalf("retry child start error = %v", err)
	}
}

func TestSessionSemanticStopPreservesOnlyXMLLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("before identity activation", func(t *testing.T) {
		t.Parallel()

		rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
		var s session
		if err := initializeSession(&s, rt, Options{MaxErrors: 1}); err != nil {
			t.Fatal(err)
		}
		if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "missing"})); err != nil {
			t.Fatalf("semantic stop start error = %v", err)
		}
		assertSemanticStopState(t, &s, "missing")
	})

	t.Run("after identity activation", func(t *testing.T) {
		t.Parallel()

		rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
    <xs:key name="byID"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`)
		var s session
		if err := initializeSession(&s, rt, Options{MaxErrors: 1}); err != nil {
			t.Fatal(err)
		}
		if err := startForTest(t, &s, 1, 1, testXMLStart(
			xml.Name{Local: "root"},
			testXMLAttr(xml.Name{Local: "unexpected"}, "value"),
		)); err != nil {
			t.Fatalf("semantic stop start error = %v", err)
		}
		assertSemanticStopState(t, &s, "root")
	})
}

func assertSemanticStopState(t *testing.T, s *session, local string) {
	t.Helper()
	if !s.doc.syntaxOnly || s.doc.Depth() != 1 || len(s.doc.errors) != 1 {
		t.Fatalf("semantic stop state: syntaxOnly=%v depth=%d errors=%d", s.doc.syntaxOnly, s.doc.Depth(), len(s.doc.errors))
	}
	if len(s.doc.identity.path) != 0 || len(s.doc.identity.elements) != 0 || s.doc.identity.startJournal.active {
		t.Fatal("semantic stop retained identity lifecycle state")
	}
	if len(s.doc.retainedPaths.nodes) != 0 || len(s.doc.retainedPaths.namespaces) != 0 {
		t.Fatal("semantic stop retained identity diagnostic paths")
	}
	if err := endForTest(t, s, 2, 1, xmlstream.EndElement{Name: xml.Name{Local: local}}); err != nil {
		t.Fatalf("syntax-only end error = %v", err)
	}
}

func TestSessionStartTransactionReusesParentTransitionStorage(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="child" type="xs:string"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	var s session
	if err := initializeSession(&s, rt, Options{}); err != nil {
		t.Fatal(err)
	}
	childEnd := xmlstream.EndElement{Name: xml.Name{Local: "child"}}
	rootEnd := xmlstream.EndElement{Name: xml.Name{Local: "root"}}
	run := func() {
		if err := startForTest(t, &s, 1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
			panic(err)
		}
		if err := startForTest(t, &s, 1, 7, testXMLStart(xml.Name{Local: "child"})); err != nil {
			panic(err)
		}
		if err := endForTest(t, &s, 1, 14, childEnd); err != nil {
			panic(err)
		}
		if err := endForTest(t, &s, 1, 22, rootEnd); err != nil {
			panic(err)
		}
		s.reset()
		sessionXMLState(&s).ready = false
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("parent transition allocations = %v, want 0 after warmup", allocs)
	}
}
