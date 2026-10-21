package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestXSIStartDiagnosticsIdentifyElement(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child" nillable="true" minOccurs="0"/><xs:element name="required" minOccurs="0"/><xs:any namespace="##other" processContents="lax" minOccurs="0" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("start.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, child, path string
		code              xsderrors.Code
	}{
		{"invalid nil", `<child xsi:nil="bad"/>`, "/root/child", xsderrors.CodeValidationNil},
		{"empty nil", `<child xsi:nil=""/>`, "/root/child", xsderrors.CodeValidationNil},
		{"empty type", `<child xsi:type=""/>`, "/root/child", xsderrors.CodeValidationType},
		{"not nillable", `<required xsi:nil="true"/>`, "/root/required", xsderrors.CodeValidationNil},
		{"wildcard type", `<o:unknown xsi:type="xs:missing"/>`, "/root/{urn:o}unknown", xsderrors.CodeValidationType},
		{"wildcard nil", `<o:unknown xsi:nil="bad"/>`, "/root/{urn:o}unknown", xsderrors.CodeValidationNil},
		{"child prefix shadow", `<o:unknown xmlns:o="urn:child" xsi:type="xs:missing"/>`, "/root/{urn:child}unknown", xsderrors.CodeValidationType},
		{"child default namespace", `<unknown xmlns="urn:child" xsi:nil="bad"/>`, "/root/{urn:child}unknown", xsderrors.CodeValidationNil},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:o="urn:o">` + "\n  " + test.child + "\n</root>"
			assertStartDiagnostics(t, engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxErrors: 1}), []startDiagnostic{{test.code, test.path, 2, 3}})
		})
	}
}

func TestXSIStartDiagnosticsPreserveNilBeforeType(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child" type="xs:string" nillable="true"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("start.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, attrs := range []string{
		`xsi:nil="bad" xsi:type="p:Missing"`,
		`xsi:type="p:Missing" xsi:nil="bad"`,
	} {
		t.Run(attrs, func(t *testing.T) {
			doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:missing">` + "\n  <child " + attrs + "/>\n</root>"
			want := []startDiagnostic{
				{xsderrors.CodeValidationNil, "/root/child", 2, 3},
				{xsderrors.CodeValidationType, "/root/child", 2, 3},
			}
			for _, maxErrors := range []int{1, 2} {
				assertStartDiagnostics(t, engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxErrors: maxErrors}), want[:maxErrors])
			}
		})
	}
}

func TestDTDDiagnosticReportedOnce(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("test.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, compileErr := xsd.Compile(xsd.Bytes("dtd.xsd", []byte("\n<!DOCTYPE xs:schema>"+schema)))
	for _, test := range []struct {
		name         string
		err          error
		path         string
		line, column int
	}{
		{"schema", compileErr, "dtd.xsd", 0, 0},
		{"instance", session.Validate(strings.NewReader("\n<!DOCTYPE root><root/>")), "/", 2, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			leaves := xsderrors.Flatten(test.err)
			if len(leaves) != 1 {
				t.Fatalf("diagnostics = %v, want one", leaves)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](leaves[0])
			if !ok {
				t.Fatalf("diagnostic = %T, want structured error", leaves[0])
			}
			if diagnostic.Category() != xsderrors.CategoryUnsupported || diagnostic.Code() != xsderrors.CodeUnsupportedDTD {
				t.Fatalf("unexpected diagnostic: %v", diagnostic)
			}
			if diagnostic.Path() != test.path || diagnostic.Line() != test.line || diagnostic.Column() != test.column {
				t.Errorf("location = %s %d:%d, want %s %d:%d", diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), test.path, test.line, test.column)
			}
			if count := strings.Count(test.err.Error(), "DTD declarations are not supported"); count != 1 {
				t.Errorf("DTD message appears %d times: %v", count, test.err)
			}
		})
	}
	if err := session.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("session reuse: %v", err)
	}
}
