package validate

import (
	"errors"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityEvaluationEnforcesOneOutstandingValueTarget(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	target, err := evaluation.prepareElementValue(1)
	if err != nil {
		t.Fatalf("prepareElementValue() error = %v", err)
	}
	if !target.needsIdentity() {
		t.Fatal("prepareElementValue() returned an inactive target")
	}
	if _, err = evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Known: true, Name: fixture.attrName}, 1); err == nil {
		t.Fatal("prepareAttributeValue() accepted an overlapping target")
	} else {
		expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
	}
	err = evaluation.rejectValue(target, identityInvalidValue, identityTestContext("/root", 0, 0))
	if err != nil {
		t.Fatalf("rejectValue() error = %v", err)
	}
	target, err = evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Known: true, Name: fixture.attrName}, 1)
	if err != nil {
		t.Fatalf("prepareAttributeValue() after release error = %v", err)
	}
	if err = evaluation.rejectValue(target, identityInvalidValue, identityTestContext("/root", 0, 0)); err != nil {
		t.Fatalf("rejectValue(attribute) error = %v", err)
	}
}

func TestIdentitySelectorUnionOverlappingBranchesSelectsNodeOnce(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
    <xs:key name="ids"><xs:selector xpath=". | ."/><xs:field xpath="@id | @*"/></xs:key>
  </xs:element>
</xs:schema>`)
	session, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Validate(strings.NewReader(`<root id="same"/>`)); err != nil {
		t.Fatalf("Validate() error = %v, want one identity selection", err)
	}
}

func TestIdentitySelectorDescendantSelfBranchUsesBroadDispatch(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
    <xs:key name="ids"><xs:selector xpath=".//."/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`)
	session, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Validate(strings.NewReader(`<root id="same"/>`)); err != nil {
		t.Fatalf("Validate() error = %v, want descendant-self selection", err)
	}
}

func TestIdentityEvaluationRejectsUnfinishedAssessment(t *testing.T) {
	t.Parallel()

	var evaluation identityEvaluation
	for _, assessment := range []identityElementAssessment{identityElementAssessmentUnknown, identityElementAssessment(99)} {
		_, err := evaluation.finishAncestorIdentitySelections(ancestorIdentityFinish{assessment: assessment}, failIdentityReport(t))
		expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
	}
}

func TestIdentityEvaluationCommitsValueAndClosesElement(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	target, err := evaluation.prepareElementValue(1)
	if err != nil {
		t.Fatalf("prepareElementValue() error = %v", err)
	}
	ctx := identityTestContext("/root", 2, 3)
	err = evaluation.recordValue(target, identityValueForTest(t, "string", "root-key", value.NeedIdentity), ctx)
	if err != nil {
		t.Fatalf("recordValue() error = %v", err)
	}
	err = evaluation.captureValue(target, ctx)
	if err != nil {
		t.Fatalf("captureValue() error = %v", err)
	}
	err = evaluation.commitValue(target)
	if err != nil {
		t.Fatalf("commitValue() error = %v", err)
	}
	result, err := evaluation.endElement(identityElementEnd{Context: ctx, ContentCaptured: true}, failIdentityReport(t))
	if err != nil {
		t.Fatalf("endElement() error = %v", err)
	}
	if result.AssessmentInvalid {
		t.Fatal("endElement() invalidated a valid identity scope")
	}
	if len(evaluation.path) != 0 || len(evaluation.elements) != 0 || len(evaluation.scopes) != 0 || len(evaluation.selections) != 0 {
		t.Fatalf("endElement() retained lifecycle state: path=%d elements=%d scopes=%d selections=%d",
			len(evaluation.path), len(evaluation.elements), len(evaluation.scopes), len(evaluation.selections))
	}
	err = evaluation.commitValue(target)
	if err == nil {
		t.Fatal("commitValue() accepted a stale target")
	} else {
		expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
	}
}

func TestIdentityEvaluationResetInvalidatesBorrowedTarget(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	target, err := evaluation.prepareElementValue(1)
	if err != nil {
		t.Fatalf("prepareElementValue() error = %v", err)
	}
	evaluation.reset(1, 1)
	err = evaluation.rejectValue(target, identityInvalidValue, identityTestContext("/root", 0, 0))
	if err == nil {
		t.Fatal("rejectValue() accepted a target borrowed before reset")
	} else {
		expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
	}
	if len(evaluation.path) != 0 || len(evaluation.elements) != 0 || evaluation.targetPhase != identityTargetInactive {
		t.Fatal("reset() retained active evaluator lifecycle state")
	}
	err = evaluation.startElement(identityElementStart{
		Context: identityTestContext("/root", 0, 0),
		Name:    xsdSchema.RuntimeName{Known: true, Name: fixture.elemName},
		Element: fixture.elemID,
		Mode:    elementAssessed,
	})
	if err != nil {
		t.Fatalf("startElement() after reset error = %v", err)
	}
}

func TestIdentityEvaluationRejectsSecondIDAttributeOnElement(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	ctx := identityTestContext("/root", 2, 3)
	target, err := evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Known: true, Name: fixture.attrName}, 1)
	if err != nil {
		t.Fatalf("prepareAttributeValue(first) error = %v", err)
	}
	err = evaluation.recordValue(target, identityValueForTest(t, "ID", "first", value.NeedIdentity), ctx)
	if err != nil {
		t.Fatalf("recordValue(first) error = %v", err)
	}
	err = evaluation.captureValue(target, ctx)
	if err != nil {
		t.Fatalf("captureValue(first) error = %v", err)
	}
	err = evaluation.commitValue(target)
	if err != nil {
		t.Fatalf("commitValue(first) error = %v", err)
	}

	target, err = evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Known: true, Name: fixture.attrName}, 1)
	if err != nil {
		t.Fatalf("prepareAttributeValue(second) error = %v", err)
	}
	err = evaluation.recordValue(target, identityValueForTest(t, "ID", "second", value.NeedIdentity), ctx)
	expectXSDCode(t, err, xsderrors.CodeValidationType)
	expectXSDMessage(t, err, "multiple ID attributes")
	if evaluation.targetPhase != identityTargetInactive {
		t.Fatal("recordValue(second) retained the rejected target")
	}
	if _, ok := evaluation.ids["second"]; ok {
		t.Fatal("recordValue(second) stored the rejected ID")
	}
}

func TestIdentityEvaluationRejectsSecondIDAfterFirstIDRecordFails(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	evaluation.limits.TupleBytes = 1
	ctx := identityTestContext("/root", 2, 3)
	target, err := evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Known: true, Name: fixture.attrName}, 1)
	if err != nil {
		t.Fatalf("prepareAttributeValue(first) error = %v", err)
	}
	err = evaluation.recordValue(target, identityValueForTest(t, "ID", "too-long", value.NeedIdentity), ctx)
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)

	target, err = evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Known: true, Name: fixture.attrName}, 1)
	if err != nil {
		t.Fatalf("prepareAttributeValue(second) error = %v", err)
	}
	err = evaluation.recordValue(target, identityValueForTest(t, "ID", "x", value.NeedIdentity), ctx)
	expectXSDCode(t, err, xsderrors.CodeValidationType)
	expectXSDMessage(t, err, "multiple ID attributes")
	if _, ok := evaluation.ids["x"]; ok {
		t.Fatal("recordValue(second) stored the rejected ID")
	}
}

func TestIdentityEvaluationCanRejectRecordedOrCapturedValue(t *testing.T) {
	for _, capture := range []bool{false, true} {
		name := "recorded"
		if capture {
			name = "captured"
		}
		t.Run(name, func(t *testing.T) {
			fixture := startedIdentityEvaluationForTest(t)
			evaluation := fixture.evaluation
			ctx := identityTestContext("/root", 2, 3)
			target, err := evaluation.prepareElementValue(1)
			if err != nil {
				t.Fatalf("prepareElementValue() error = %v", err)
			}
			err = evaluation.recordValue(target, identityValueForTest(t, "string", "root-key", value.NeedIdentity), ctx)
			if err != nil {
				t.Fatalf("recordValue() error = %v", err)
			}
			if capture {
				err = evaluation.captureValue(target, ctx)
				if err != nil {
					t.Fatalf("captureValue() error = %v", err)
				}
			}
			err = evaluation.rejectValue(target, identityInvalidValue, ctx)
			if err != nil {
				t.Fatalf("rejectValue() error = %v", err)
			}
			if evaluation.targetPhase != identityTargetInactive {
				t.Fatal("rejectValue() retained the rejected target")
			}
			if evaluation.fieldValues[0].state != identityFieldInvalid {
				t.Fatal("rejectValue() did not invalidate the matched field")
			}
		})
	}
}

func TestIdentityEvaluationStartTransactionRollsBackState(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	evaluation.maxScopes = len(evaluation.scopes)
	pathLen := len(evaluation.path)
	elementLen := len(evaluation.elements)
	scopeLen := len(evaluation.scopes)
	selectionLen := len(evaluation.selections)
	fieldLen := len(evaluation.fieldValues)
	if err := evaluation.beginStart(); err != nil {
		t.Fatal(err)
	}
	err := evaluation.startElement(identityElementStart{
		Context: identityTestContext("/root/child", 2, 3),
		Name:    xsdSchema.RuntimeName{Known: true, Name: fixture.elemName},
		Element: fixture.elemID,
		Mode:    elementAssessed,
	})
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	evaluation.abortStart()
	if len(evaluation.path) != pathLen || len(evaluation.elements) != elementLen ||
		len(evaluation.scopes) != scopeLen || len(evaluation.selections) != selectionLen ||
		len(evaluation.fieldValues) != fieldLen {
		t.Fatalf("aborted identity start retained state: path=%d elements=%d scopes=%d selections=%d fields=%d",
			len(evaluation.path), len(evaluation.elements), len(evaluation.scopes), len(evaluation.selections), len(evaluation.fieldValues))
	}
}

func TestIdentityEvaluationStartTransactionRestoresPreexistingMutations(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	evaluation.fieldValues[0] = identityFieldValue{value: "original", state: identityFieldPresent}
	if err := evaluation.beginStart(); err != nil {
		t.Fatal(err)
	}
	err := evaluation.captureFields(
		[]identityFieldMatch{{Selection: 0, Field: 0}},
		identityFieldValue{value: "duplicate", state: identityFieldPresent},
		identityTestContext("/root/child", 2, 3),
	)
	expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
	if !evaluation.scopes[0].invalid || evaluation.fieldValues[0].state != identityFieldInvalid {
		t.Fatal("test mutation did not invalidate the pre-existing field and scope")
	}
	evaluation.abortStart()
	if got := evaluation.fieldValues[0]; got != (identityFieldValue{value: "original", state: identityFieldPresent}) {
		t.Fatalf("abort restored field = %+v, want original present value", got)
	}
	if evaluation.scopes[0].invalid {
		t.Fatal("abort retained pre-existing scope invalidation")
	}
}

func TestIdentityEvaluationRecordsIDBatchAtomically(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	evaluation.limits.Entries = 1
	target, err := evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Known: true, Name: fixture.attrName}, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = evaluation.recordValue(target, identityValueForTest(t, "IDREFS", "one two", value.NeedIdentity), identityTestContext("/root", 0, 0))
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	if len(evaluation.ids) != 0 || evaluation.entries != 0 {
		t.Fatalf("failed ID batch changed identity state: ids=%v entries=%d", evaluation.ids, evaluation.entries)
	}
	if len(evaluation.fieldStaging.ids) != 0 || len(evaluation.fieldStaging.values) != 0 {
		t.Fatalf("failed ID batch retained staging: %+v", evaluation.fieldStaging)
	}
}

func TestIdentityEvaluationFinishesOnlySelectionsAtCurrentDepth(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	if len(evaluation.selections) != 1 {
		t.Fatalf("started selections = %d, want 1", len(evaluation.selections))
	}
	shallower := evaluation.selections[0]
	if err := evaluation.startSelection(
		shallower.scope,
		shallower.depth+1,
		shallower.constraint,
		shallower.fieldLen,
		0,
		identityTestContext("/root/child", 2, 1),
	); err != nil {
		t.Fatalf("startSelection() error = %v", err)
	}
	if err := evaluation.finishSelections(
		shallower.depth+1,
		identityTestContext("/root/child", 2, 1),
		identitySelectionsOwnedByAncestorScope,
		func(error) error { return nil },
	); err != nil {
		t.Fatalf("finishSelections() error = %v", err)
	}
	if len(evaluation.selections) != 1 || evaluation.selections[0] != shallower {
		t.Fatalf("remaining selections = %+v, want shallower selection %+v", evaluation.selections, shallower)
	}
}

type startedIdentityEvaluationFixture struct {
	evaluation *identityEvaluation
	elemID     xsdSchema.ElementID
	elemName   xsdSchema.QName
	attrName   xsdSchema.QName
}

func startedIdentityEvaluationForTest(t *testing.T) startedIdentityEvaluationFixture {
	t.Helper()

	fixture := compiledIdentityRuntimeForTest(t)
	evaluation := newIdentityEvaluation(fixture.rt, identityLimits{}, 0)
	if err := evaluation.startElement(identityElementStart{
		Context: identityTestContext("/root", 1, 1),
		Name:    xsdSchema.RuntimeName{Known: true, Name: fixture.elemName},
		Element: fixture.elemID,
		Mode:    elementAssessed,
	}); err != nil {
		t.Fatalf("startElement() error = %v", err)
	}
	return startedIdentityEvaluationFixture{
		evaluation: &evaluation,
		elemID:     fixture.elemID,
		elemName:   fixture.elemName,
		attrName:   fixture.attrName,
	}
}

func TestIdentityReporterAbortPreservesPendingSelections(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="row">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string"/>
      <xs:attribute name="id" type="xs:string"/>
    </xs:complexType>
  </xs:element>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element ref="row" maxOccurs="unbounded"/></xs:sequence>
    </xs:complexType>
    <xs:key name="aKey"><xs:selector xpath="row"/><xs:field xpath="@a"/></xs:key>
    <xs:key name="idKey"><xs:selector xpath="row"/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`)
	evaluation := newIdentityEvaluation(rt, identityLimits{}, 0)
	startIdentityRetryElement(t, &evaluation, "root", identityTestContext("/root", 1, 1))

	first := identityTestContext("/root/row[1]", 2, 1)
	startIdentityRetryElement(t, &evaluation, "row", first)
	captureIdentityRetryAttribute(t, &evaluation, "a", "first-a", first)
	captureIdentityRetryAttribute(t, &evaluation, "id", "shared-id", first)
	if _, err := evaluation.endElement(identityElementEnd{Context: first}, failIdentityReport(t)); err != nil {
		t.Fatalf("endElement(first row) error = %v", err)
	}

	second := identityTestContext("/root/row[2]", 3, 1)
	startIdentityRetryElement(t, &evaluation, "row", second)
	captureIdentityRetryAttribute(t, &evaluation, "id", "shared-id", second)

	abort := errors.New("reporter stopped")
	var reported []error
	err := evaluation.finishSelections(2, second, identitySelectionsOwnedByAncestorScope, func(err error) error {
		reported = append(reported, err)
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("finishSelections(abort) error = %v, want %v", err, abort)
	}
	if len(reported) != 1 {
		t.Fatalf("abort diagnostics = %v, want one missing-key diagnostic", reported)
	}
	expectXSDCode(t, reported[0], xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, reported[0], "key field is missing")
	expectXSDLocation(t, reported[0], "/root/row[2]", 3, 1)

	reported = nil
	err = evaluation.finishSelections(2, second, identitySelectionsOwnedByAncestorScope, func(err error) error {
		reported = append(reported, err)
		return nil
	})
	if err != nil {
		t.Fatalf("finishSelections(retry) error = %v", err)
	}
	if len(reported) != 1 {
		t.Fatalf("retry diagnostics = %v, want only the pending duplicate-key diagnostic", reported)
	}
	expectXSDCode(t, reported[0], xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, reported[0], "duplicate identity value first seen at /root/row[1]")
	expectXSDLocation(t, reported[0], "/root/row[2]", 3, 1)

	if err := evaluation.finishSelections(2, second, identitySelectionsOwnedByAncestorScope, failIdentityReport(t)); err != nil {
		t.Fatalf("finishSelections(after retry) error = %v", err)
	}
}

func startIdentityRetryElement(tb testing.TB, evaluation *identityEvaluation, local string, ctx StartContext) {
	tb.Helper()
	name, ok := evaluation.rt.LookupQName("", local)
	if !ok {
		tb.Fatalf("LookupQName(%q) failed", local)
	}
	runtimeName := xsdSchema.RuntimeName{Known: true, Name: name}
	element, _, ok := evaluation.rt.RootElement(runtimeName)
	if !ok {
		tb.Fatalf("RootElement(%q) failed", local)
	}
	if err := evaluation.startElement(identityElementStart{
		Context: ctx,
		Name:    runtimeName,
		Element: element,
		Mode:    elementAssessed,
	}); err != nil {
		tb.Fatalf("startElement(%q) error = %v", local, err)
	}
}

func captureIdentityRetryAttribute(tb testing.TB, evaluation *identityEvaluation, local, lexical string, ctx StartContext) {
	tb.Helper()
	name, ok := evaluation.rt.LookupQName("", local)
	if !ok {
		tb.Fatalf("LookupQName(%q) failed", local)
	}
	target, err := evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Known: true, Name: name}, 2)
	if err != nil {
		tb.Fatalf("prepareAttributeValue(%q) error = %v", local, err)
	}
	if err := evaluation.recordValue(target, identityValueForTest(tb, "string", lexical, value.NeedIdentity), ctx); err != nil {
		tb.Fatalf("recordValue(%q) error = %v", local, err)
	}
	if err := evaluation.captureValue(target, ctx); err != nil {
		tb.Fatalf("captureValue(%q) error = %v", local, err)
	}
	if err := evaluation.commitValue(target); err != nil {
		tb.Fatalf("commitValue(%q) error = %v", local, err)
	}
}

func TestIdentityNilledFieldsHaveNoTupleValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		kind       string
		body       string
		message    string
		complexNil bool
	}{
		{name: "unique ignores nilled rows", kind: "unique", body: `<row><v xsi:nil="true"/></row><row><v xsi:nil="true"/></row>`},
		{name: "nil differs from empty string", kind: "unique", body: `<row><v xsi:nil="true"/></row><row><v/></row>`},
		{name: "empty strings duplicate", kind: "unique", body: `<row><v/></row><row><v/></row>`, message: "duplicate identity value first seen at /root/row"},
		{name: "nil then value selects two nodes", kind: "unique", body: `<row><v xsi:nil="true"/><v>x</v></row>`, message: "identity field selects multiple values"},
		{name: "value then nil selects two nodes", kind: "unique", body: `<row><v>x</v><v xsi:nil="true"/></row>`, message: "identity field selects multiple values"},
		{name: "two nils select two nodes", kind: "unique", body: `<row><v xsi:nil="true"/><v xsi:nil="true"/></row>`, message: "identity field selects multiple values"},
		{name: "key nil has no value", kind: "key", body: `<row><v xsi:nil="true"/></row>`, message: "key field is missing"},
		{name: "key still rejects nillable declaration", kind: "key", body: `<row><v>x</v></row>`, message: "key field selects nillable element declaration"},
		{name: "keyref ignores nil", kind: "keyref", body: `<row><v xsi:nil="true"/></row>`},
		{name: "keyref physical value resolves", kind: "keyref", body: `<target id="x"/><row><v>x</v></row>`},
		{name: "keyref physical value is unresolved", kind: "keyref", body: `<row><v xsi:nil="true"/></row><row><v>x</v></row>`, message: "keyref does not resolve"},
		{name: "nil does not waive field type", kind: "unique", complexNil: true, body: `<row><v xsi:nil="true"/></row>`, message: "identity field has no simple value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			field := `<xs:element name="v" type="xs:string" nillable="true" minOccurs="0" maxOccurs="2"/>`
			if test.complexNil {
				field = `<xs:element name="v" nillable="true" minOccurs="0" maxOccurs="2"><xs:complexType/></xs:element>`
			}
			refer := ""
			if test.kind == "keyref" {
				refer = ` refer="targets"`
			}
			text := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence>
<xs:element name="target" minOccurs="0" maxOccurs="unbounded"><xs:complexType><xs:attribute name="id" type="xs:string" use="required"/></xs:complexType></xs:element>
<xs:element name="row" minOccurs="0" maxOccurs="unbounded"><xs:complexType><xs:sequence>` + field + `</xs:sequence></xs:complexType></xs:element>
</xs:sequence></xs:complexType>
<xs:key name="targets"><xs:selector xpath="target"/><xs:field xpath="@id"/></xs:key>
<xs:` + test.kind + ` name="values"` + refer + `><xs:selector xpath="row"/><xs:field xpath="v"/></xs:` + test.kind + `>
</xs:element></xs:schema>`
			rt := compileRuntimeForTest(t, text)
			session, err := newSessionForTest(rt, Options{})
			if err != nil {
				t.Fatal(err)
			}
			err = session.Validate(strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` + test.body + `</root>`))
			if test.message == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
			expectXSDMessage(t, err, test.message)
		})
	}
}
