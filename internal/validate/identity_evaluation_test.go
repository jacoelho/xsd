package validate

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityEvaluationEnforcesOneOutstandingValueTarget(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	target, err := evaluation.prepareElementValue()
	if err != nil {
		t.Fatalf("prepareElementValue() error = %v", err)
	}
	if !target.needsIdentity() {
		t.Fatal("prepareElementValue() returned an inactive target")
	}
	if _, err = evaluation.prepareAttributeValue(runtime.RuntimeName{Known: true, Name: fixture.attrName}); err == nil {
		t.Fatal("prepareAttributeValue() accepted an overlapping target")
	} else {
		expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
	}
	err = evaluation.rejectValue(target, identityInvalidValue, identityTestContext("/root", 0, 0))
	if err != nil {
		t.Fatalf("rejectValue() error = %v", err)
	}
	target, err = evaluation.prepareAttributeValue(runtime.RuntimeName{Known: true, Name: fixture.attrName})
	if err != nil {
		t.Fatalf("prepareAttributeValue() after release error = %v", err)
	}
	if err = evaluation.rejectValue(target, identityInvalidValue, identityTestContext("/root", 0, 0)); err != nil {
		t.Fatalf("rejectValue(attribute) error = %v", err)
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
	target, err := evaluation.prepareElementValue()
	if err != nil {
		t.Fatalf("prepareElementValue() error = %v", err)
	}
	ctx := identityTestContext("/root", 2, 3)
	err = evaluation.recordValue(target, runtime.SimpleValue{Identity: "root-key"}, ctx)
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
	target, err := evaluation.prepareElementValue()
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
		Name:    runtime.RuntimeName{Known: true, Name: fixture.elemName},
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
	target, err := evaluation.prepareAttributeValue(runtime.RuntimeName{Known: true, Name: fixture.attrName})
	if err != nil {
		t.Fatalf("prepareAttributeValue(first) error = %v", err)
	}
	err = evaluation.recordValue(target, runtime.SimpleValue{IDs: "first", Identity: "first"}, ctx)
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

	target, err = evaluation.prepareAttributeValue(runtime.RuntimeName{Known: true, Name: fixture.attrName})
	if err != nil {
		t.Fatalf("prepareAttributeValue(second) error = %v", err)
	}
	err = evaluation.recordValue(target, runtime.SimpleValue{IDs: "second", Identity: "second"}, ctx)
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
	target, err := evaluation.prepareAttributeValue(runtime.RuntimeName{Known: true, Name: fixture.attrName})
	if err != nil {
		t.Fatalf("prepareAttributeValue(first) error = %v", err)
	}
	err = evaluation.recordValue(target, runtime.SimpleValue{IDs: "too-long", Identity: "too-long"}, ctx)
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)

	target, err = evaluation.prepareAttributeValue(runtime.RuntimeName{Known: true, Name: fixture.attrName})
	if err != nil {
		t.Fatalf("prepareAttributeValue(second) error = %v", err)
	}
	err = evaluation.recordValue(target, runtime.SimpleValue{IDs: "x", Identity: "x"}, ctx)
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
			target, err := evaluation.prepareElementValue()
			if err != nil {
				t.Fatalf("prepareElementValue() error = %v", err)
			}
			err = evaluation.recordValue(target, runtime.SimpleValue{Identity: "root-key"}, ctx)
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
		Name:    runtime.RuntimeName{Known: true, Name: fixture.elemName},
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
		"duplicate",
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
	target, err := evaluation.prepareAttributeValue(runtime.RuntimeName{Known: true, Name: fixture.attrName})
	if err != nil {
		t.Fatal(err)
	}
	err = evaluation.recordValue(target, runtime.SimpleValue{IDs: "one two", Identity: "value"}, identityTestContext("/root", 0, 0))
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
	elemID     runtime.ElementID
	elemName   runtime.QName
	attrName   runtime.QName
}

func startedIdentityEvaluationForTest(t *testing.T) startedIdentityEvaluationFixture {
	t.Helper()

	fixture := compiledIdentityRuntimeForTest(t)
	evaluation := newIdentityEvaluation(fixture.rt, identityLimits{}, 0)
	if err := evaluation.startElement(identityElementStart{
		Context: identityTestContext("/root", 1, 1),
		Name:    runtime.RuntimeName{Known: true, Name: fixture.elemName},
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
