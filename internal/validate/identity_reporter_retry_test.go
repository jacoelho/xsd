package validate

import (
	"errors"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

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
