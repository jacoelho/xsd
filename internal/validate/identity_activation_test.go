package validate

import (
	"encoding/xml"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityEvaluationActivatesDocumentIdentityAtValidatedValueDepth(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)
	evaluation := newIdentityEvaluation(rt, identityLimits{}, 0)
	if evaluation.constraintsEnabled {
		t.Fatal("constraint-free schema unexpectedly enabled key identity")
	}
	if err := evaluation.startElement(identityElementStart{
		Context: identityTestContext("/root", 1, 1),
		Mode:    elementAssessed,
	}); err != nil {
		t.Fatalf("startElement() error = %v", err)
	}
	if evaluation.documentIdentityActive || len(evaluation.elements) != 0 {
		t.Fatalf("dormant identity state = active %t, elements=%d; want inactive and empty", evaluation.documentIdentityActive, len(evaluation.elements))
	}

	target, err := evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Local: "id"}, 1)
	if err != nil {
		t.Fatalf("prepareAttributeValue() error = %v", err)
	}
	if target.depth != 1 || target.needsIdentity() {
		t.Fatalf("target = %+v, want depth 1 and no key-field match", target)
	}
	if err := evaluation.recordValue(target, identityValueForTest(t, "ID", "root-id", value.NeedIdentity), identityTestContext("/root", 1, 5)); err != nil {
		t.Fatalf("recordValue() error = %v", err)
	}
	if !evaluation.documentIdentityActive || len(evaluation.elements) != 1 {
		t.Fatalf("activated identity state = active %t, elements=%d; want active with one frame", evaluation.documentIdentityActive, len(evaluation.elements))
	}
	if _, ok := evaluation.ids["root-id"]; !ok {
		t.Fatal("recordValue() did not retain validated ID")
	}

	if err := evaluation.commitValue(target); err != nil {
		t.Fatalf("commitValue() error = %v", err)
	}
	if _, err := evaluation.endElement(identityElementEnd{
		Context:         identityTestContext("/root", 1, 10),
		ContentCaptured: true,
	}, failIdentityReport(t)); err != nil {
		t.Fatalf("endElement() error = %v", err)
	}
	if len(evaluation.elements) != 0 {
		t.Fatalf("endElement() retained identity frames = %d, want 0", len(evaluation.elements))
	}
}

func TestIdentityEvaluationFirstDocumentIdentityActivationRollsBackOnFatalStart(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)
	evaluation := newIdentityEvaluation(rt, identityLimits{TupleBytes: 1}, 0)
	if err := evaluation.beginStart(); err != nil {
		t.Fatalf("beginStart() error = %v", err)
	}
	if err := evaluation.startElement(identityElementStart{
		Context: identityTestContext("/root", 1, 1),
		Mode:    elementAssessed,
	}); err != nil {
		t.Fatalf("startElement() error = %v", err)
	}
	target, err := evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Local: "id"}, 1)
	if err != nil {
		t.Fatalf("prepareAttributeValue() error = %v", err)
	}
	err = evaluation.recordValue(target, identityValueForTest(t, "ID", "too-long", value.NeedIdentity), identityTestContext("/root", 1, 5))
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	if !evaluation.documentIdentityActive {
		t.Fatal("first identity record did not activate state before the fatal limit")
	}

	evaluation.abortStart()
	if evaluation.documentIdentityActive {
		t.Fatal("abortStart() retained first document identity activation")
	}
	if len(evaluation.elements) != 0 || len(evaluation.ids) != 0 || len(evaluation.idrefs) != 0 || evaluation.entries != 0 {
		t.Fatalf("abortStart() retained identity state: elements=%d ids=%d refs=%d entries=%d", len(evaluation.elements), len(evaluation.ids), len(evaluation.idrefs), evaluation.entries)
	}
}

func TestIdentityEvaluationLazyIDREFStillRunsDocumentFinalCheck(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)
	evaluation := newIdentityEvaluation(rt, identityLimits{}, 0)
	if err := evaluation.startElement(identityElementStart{
		Context: identityTestContext("/root", 1, 1),
		Mode:    elementAssessed,
	}); err != nil {
		t.Fatalf("startElement() error = %v", err)
	}
	target, err := evaluation.prepareElementValue(1)
	if err != nil {
		t.Fatalf("prepareElementValue() error = %v", err)
	}
	if err := evaluation.recordValue(target, identityValueForTest(t, "IDREF", "missing", value.NeedIdentity), identityTestContext("/root", 2, 3)); err != nil {
		t.Fatalf("recordValue() error = %v", err)
	}
	if err := evaluation.commitValue(target); err != nil {
		t.Fatalf("commitValue() error = %v", err)
	}
	if _, err := evaluation.endElement(identityElementEnd{
		Context:         identityTestContext("/root", 2, 3),
		ContentCaptured: true,
	}, failIdentityReport(t)); err != nil {
		t.Fatalf("endElement() error = %v", err)
	}
	var got error
	if err := evaluation.endDocument(func(err error) error {
		got = err
		return nil
	}); err != nil {
		t.Fatalf("endDocument() error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationType)
	expectXSDMessage(t, got, "IDREF does not resolve: missing")
}

func TestSessionStartRollsBackFirstLazyDocumentIdentityActivation(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="child">
      <xs:complexType>
        <xs:attribute name="id" type="xs:ID"/>
        <xs:attribute name="ref" type="xs:IDREF"/>
      </xs:complexType>
    </xs:element></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)
	var s session
	if err := initializeSessionForTest(&s, rt, Options{MaxIdentityEntries: 1}); err != nil {
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

	err := startForTest(t, &s, 2, 1, testXMLStart(
		xml.Name{Local: "child"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:failed"),
		testXMLAttr(xml.Name{Local: "id"}, "one"),
		testXMLAttr(xml.Name{Local: "ref"}, "two"),
	))
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	parent, ok = s.doc.Current()
	if !ok || s.doc.Depth() != 1 || *parent != parentBefore {
		t.Fatalf("failed child changed XML state: depth=%d frame=%+v want=%+v", s.doc.Depth(), parent, parentBefore)
	}
	identity := &s.doc.identity
	if identity.documentIdentityActive || len(identity.elements) != 0 || len(identity.path) != 0 || len(identity.ids) != 0 || len(identity.idrefs) != 0 || identity.entries != 0 || identity.startJournal.active {
		t.Fatalf("failed child retained lazy identity state: active=%t elements=%d path=%d ids=%d refs=%d entries=%d journal=%t", identity.documentIdentityActive, len(identity.elements), len(identity.path), len(identity.ids), len(identity.idrefs), identity.entries, identity.startJournal.active)
	}
	if len(s.doc.retainedPaths.nodes) != 0 || len(s.doc.retainedPaths.namespaces) != 0 {
		t.Fatalf("failed child retained diagnostic paths: nodes=%d namespaces=%d", len(s.doc.retainedPaths.nodes), len(s.doc.retainedPaths.namespaces))
	}
	if _, ok := s.reader.Lookup("p"); ok {
		t.Fatal("failed child retained namespace binding")
	}

	if err := startForTest(t, &s, 2, 1, testXMLStart(
		xml.Name{Local: "child"},
		testXMLAttr(xml.Name{Local: "id"}, "one"),
	)); err != nil {
		t.Fatalf("retry child start error = %v", err)
	}
	if !identity.documentIdentityActive || len(identity.elements) != 2 {
		t.Fatalf("retry child identity state = active %t, elements=%d; want active with XML depth 2", identity.documentIdentityActive, len(identity.elements))
	}
}
