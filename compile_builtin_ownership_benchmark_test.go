package xsd_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const builtinOwnershipSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="L"><xs:list itemType="xs:int"/></xs:simpleType>
  <xs:simpleType name="U"><xs:union memberTypes="xs:boolean L"/></xs:simpleType>
  <xs:simpleType name="Amount"><xs:restriction base="xs:decimal"><xs:minInclusive value="1.5"/></xs:restriction></xs:simpleType>
  <xs:complexType name="TextBase"><xs:simpleContent><xs:extension base="xs:string"><xs:attribute name="note"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:complexType name="Text3"><xs:simpleContent><xs:restriction base="TextBase"><xs:maxLength value="3"/></xs:restriction></xs:simpleContent></xs:complexType>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element name="l" type="L"/><xs:element name="u" type="U"/><xs:element name="amount" type="Amount"/><xs:element name="text" type="Text3"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`

const builtinOwnershipValid = `<root><l>1 2</l><u>true</u><amount>2.0</amount><text note="raw p:q">abc</text></root>`

func checkBuiltinOwnership(tb testing.TB, engine *xsd.Engine) {
	tb.Helper()
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	reader := bytes.NewReader(nil)
	for _, document := range []string{
		builtinOwnershipValid,
		`<root><l>1 2</l><u>1 2</u><amount>2.0</amount><text note="raw p:q">abc</text></root>`,
	} {
		reader.Reset([]byte(document))
		if err := session.Validate(reader); err != nil {
			tb.Fatalf("valid builtin routes: %v", err)
		}
	}
	for _, opponent := range []struct {
		name     string
		document string
		message  string
	}{
		{"list", `<root><l>1 x</l><u>true</u><amount>2.0</amount><text note="raw p:q">abc</text></root>`, "invalid simple content: invalid decimal"},
		{"union", `<root><l>1 2</l><u>not-a-value</u><amount>2.0</amount><text note="raw p:q">abc</text></root>`, "invalid simple content: invalid boolean"},
		{"bound", `<root><l>1 2</l><u>true</u><amount>1.0</amount><text note="raw p:q">abc</text></root>`, "invalid simple content: minInclusive facet failed"},
		{"length", `<root><l>1 2</l><u>true</u><amount>2.0</amount><text note="raw p:q">abcd</text></root>`, "invalid simple content: length facet failed"},
	} {
		reader.Reset([]byte(opponent.document))
		err := session.Validate(reader)
		if err == nil || !xsderrors.IsConclusiveValidation(err) {
			tb.Fatalf("%s opponent requires semantic rejection: %v", opponent.name, err)
		}
		failures := xsderrors.Flatten(err)
		if len(failures) != 1 {
			tb.Fatalf("%s diagnostic count = %d, want 1: %v", opponent.name, len(failures), err)
		}
		diagnostic, ok := errors.AsType[*xsderrors.Error](failures[0])
		if !ok || diagnostic.Category() != xsderrors.CategoryValidation || diagnostic.Code() != xsderrors.CodeValidationFacet || diagnostic.Message() != opponent.message {
			tb.Fatalf("%s diagnostic = %v", opponent.name, err)
		}
		reader.Reset([]byte(builtinOwnershipValid))
		if recoveryErr := session.Validate(reader); recoveryErr != nil {
			tb.Fatalf("same-session recovery after %s: %v", opponent.name, recoveryErr)
		}
	}
}

func TestCompileBuiltinSimpleRoutesSemantics(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("builtin-ownership.xsd", []byte(builtinOwnershipSchema)))
	if err != nil || engine == nil {
		t.Fatalf("compile builtin routes: engine=%v err=%v", engine, err)
	}
	checkBuiltinOwnership(t, engine)
}

func BenchmarkCompileBuiltinSimpleRoutes(b *testing.B) {
	source := xsd.Bytes("builtin-ownership.xsd", []byte(builtinOwnershipSchema))
	engine, err := xsd.Compile(source)
	if err != nil || engine == nil {
		b.Fatalf("compile builtin routes: engine=%v err=%v", engine, err)
	}
	checkBuiltinOwnership(b, engine)
	b.ReportAllocs()
	for b.Loop() {
		engine, err = xsd.Compile(source)
		if err != nil || engine == nil {
			b.Fatalf("compile builtin routes: engine=%v err=%v", engine, err)
		}
	}
	checkBuiltinOwnership(b, engine)
}
