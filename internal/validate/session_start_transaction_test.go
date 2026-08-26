package validate

import (
	"encoding/xml"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSessionStartRollsBackRootOnFatalAssessment(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	var s session
	if err := initializeSession(&s, rt, Options{}); err != nil {
		t.Fatal(err)
	}
	err := s.start(1, 1, testXMLStart(
		xml.Name{Local: "missing"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "xsi"}, vocab.XSINamespaceURI),
		testXMLAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrNoNamespaceSchemaLocation}, "missing.xsd"),
	))
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
	if s.doc.Depth() != 0 || s.doc.seenRoot || s.doc.schemaLocationHints.Has("") {
		t.Fatalf("failed root start changed document state: depth=%d seenRoot=%v hints=%+v", s.doc.Depth(), s.doc.seenRoot, s.doc.schemaLocationHints)
	}
	if len(s.doc.identity.path) != 0 || len(s.doc.identity.elements) != 0 {
		t.Fatal("failed root start changed identity state")
	}
	if _, ok := s.doc.LookupNamespace("xsi"); ok {
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
	if err := s.start(1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	parent, ok := s.doc.Current()
	if !ok {
		t.Fatal("root start did not create a frame")
	}
	parentBefore := *parent
	err := s.start(2, 1, stream.OwnedStartElement(
		xml.Name{Local: "child"},
		stream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "xsi"}, vocab.XSINamespaceURI),
		stream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:missing"),
		stream.OwnedAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrSchemaLocation}, "urn:missing missing.xsd"),
		stream.OwnedAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrType}, "p:Missing"),
	))
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
	parent, ok = s.doc.Current()
	if !ok || s.doc.Depth() != 1 || *parent != parentBefore {
		t.Fatalf("failed child start changed parent: depth=%d frame=%+v want=%+v", s.doc.Depth(), parent, parentBefore)
	}
	if s.doc.schemaLocationHints.Has("urn:missing") {
		t.Fatal("failed child start retained schema-location hint")
	}
	if _, ok := s.doc.LookupNamespace("p"); ok {
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
	if err := s.start(1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	allBitsBefore := slices.Clone(s.doc.allBits)
	err := s.start(2, 1, stream.OwnedStartElement(
		xml.Name{Local: "child"},
		stream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "xsi"}, vocab.XSINamespaceURI),
		stream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:missing"),
		stream.OwnedAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrSchemaLocation}, "urn:missing missing.xsd"),
		stream.OwnedAttr(xml.Name{Space: "xsi", Local: vocab.XSIAttrType}, "p:Missing"),
	))
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
	if !slices.Equal(s.doc.allBits, allBitsBefore) {
		t.Fatalf("failed child start changed xs:all bits: got %v want %v", s.doc.allBits, allBitsBefore)
	}
	if err := s.start(2, 1, testXMLStart(xml.Name{Local: "child"})); err != nil {
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
	if err := s.start(1, 1, testXMLStart(
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

	err := s.start(2, 1, testXMLStart(
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
	if _, ok := s.doc.LookupNamespace("p"); ok {
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
	if err := s.start(1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	checkpoint := s.doc.startCheckpoint()
	prepared, err := s.doc.PrepareStart(testXMLStart(xml.Name{Local: "unexpected"}), &s.valueStrings, s.limits.InstanceDepth, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := s.beginStartTransaction(checkpoint, prepared.namespace)
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

	if err := s.start(2, 1, testXMLStart(xml.Name{Local: "unexpected"})); err != nil {
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
	end := stream.EndElement{Name: xml.Name{Local: "root"}}
	run := func() {
		if err := s.start(1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
			panic(err)
		}
		if err := s.end(1, 8, end); err != nil {
			panic(err)
		}
		s.reset()
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
	if err := s.start(1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
		t.Fatal(err)
	}
	parent, ok := s.doc.Current()
	if !ok {
		t.Fatal("root start did not create a frame")
	}
	parentBefore := *parent
	allBitsBefore := slices.Clone(s.doc.allBits)

	err := s.start(2, 1, testXMLStart(
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
	childEnd := stream.EndElement{Name: xml.Name{Local: "child"}}
	rootEnd := stream.EndElement{Name: xml.Name{Local: "root"}}
	run := func() {
		if err := s.start(1, 1, testXMLStart(xml.Name{Local: "root"})); err != nil {
			panic(err)
		}
		if err := s.start(1, 7, testXMLStart(xml.Name{Local: "child"})); err != nil {
			panic(err)
		}
		if err := s.end(1, 14, childEnd); err != nil {
			panic(err)
		}
		if err := s.end(1, 22, rootEnd); err != nil {
			panic(err)
		}
		s.reset()
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("parent transition allocations = %v, want 0 after warmup", allocs)
	}
}
