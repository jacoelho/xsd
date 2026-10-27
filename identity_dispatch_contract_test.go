package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityElementBranchesCaptureEachNodeOnce(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">
<xs:element name="root"><xs:complexType><xs:sequence><xs:element name="v" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
<xs:unique name="once"><xs:selector xpath="."/><xs:field xpath="p:v | p:* | * | p:v"/></xs:unique></xs:element></xs:schema>`
	engine := compileIdentityDispatchTestEngine(t, schema)
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const valid = `<root xmlns="urn:t"><v>x</v></root>`
	if validateErr := session.Validate(strings.NewReader(valid)); validateErr != nil {
		t.Fatalf("overlapping branches counted one node more than once: %v", validateErr)
	}
	err = session.Validate(strings.NewReader(`<root xmlns="urn:t"><v>x</v><v>y</v></root>`))
	failures := xsderrors.Flatten(err)
	if len(failures) != 1 {
		t.Fatalf("distinct nodes: got %v, want one identity diagnostic", err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](failures[0])
	if !ok || diagnostic.Code() != xsderrors.CodeValidationIdentity || !strings.Contains(diagnostic.Message(), "selects multiple nodes; at most one is allowed") {
		t.Fatalf("distinct selected nodes diagnostic: %v", err)
	}
	if validateErr := session.Validate(strings.NewReader(valid)); validateErr != nil {
		t.Fatalf("reuse after cardinality failure: %v", validateErr)
	}
}

func TestIdentityElementFieldsKeepRecursiveSelectionsSeparate(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:element name="root"><xs:complexType><xs:choice minOccurs="0" maxOccurs="unbounded"><xs:element ref="root"/><xs:element name="row" type="xs:string"/></xs:choice></xs:complexType>
<xs:unique name="u"><xs:selector xpath=".//row"/><xs:field xpath="."/></xs:unique></xs:element></xs:schema>`
	engine := compileIdentityDispatchTestEngine(t, schema)
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const valid = `<root><root><row>x</row></root><root><row>y</row></root></root>`
	if validateErr := session.Validate(strings.NewReader(valid)); validateErr != nil {
		t.Fatalf("distinct recursive rows: %v", validateErr)
	}
	err = session.Validate(strings.NewReader(`<root><root><row>x</row></root><root><row>x</row></root></root>`))
	failures := xsderrors.Flatten(err)
	if len(failures) != 1 {
		t.Fatalf("ancestor duplicate: got %v, want one identity diagnostic", err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](failures[0])
	if !ok || diagnostic.Code() != xsderrors.CodeValidationIdentity || diagnostic.Message() != `duplicate value for identity constraint "u"; first seen at /root/root/row` {
		t.Fatalf("ancestor duplicate diagnostic: %v", err)
	}
	if validateErr := session.Validate(strings.NewReader(valid)); validateErr != nil {
		t.Fatalf("reuse after recursive duplicate: %v", validateErr)
	}
}

func compileIdentityDispatchTestEngine(t *testing.T, schema string) *xsd.Engine {
	t.Helper()
	engine, err := xsd.Compile(xsd.Bytes("identity.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	return engine
}
