package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestPresentEmptyFixedElement(t *testing.T) {
	t.Parallel()
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:string" fixed=""/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("empty-fixed.xsd", []byte(source)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, document := range []string{`<r/>`, `<r></r>`} {
		if err := session.Validate(strings.NewReader(document)); err != nil {
			t.Fatalf("accepted document %q: %v", document, err)
		}
	}
	expectCategoryCode(t, session.Validate(strings.NewReader(`<r>x</r>`)), xsderrors.CategoryValidation, xsderrors.CodeValidationElement)
	if err := session.Validate(strings.NewReader(`<r/>`)); err != nil {
		t.Fatalf("same-session recovery: %v", err)
	}
}

func TestElementConstraintConflictPrecedesIdentityXPath(t *testing.T) {
	t.Parallel()
	const conflict = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r" type="xs:string" default="" fixed="">
    <xs:key name="value"><xs:selector xpath="["/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`
	for _, test := range []struct {
		name, source, message string
		code                  xsderrors.Code
		line, column          int
	}{
		{"conflict", conflict, "element cannot have both default and fixed", xsderrors.CodeSchemaInvalidAttribute, 2, 3},
		{"xpath", strings.Replace(conflict, ` default=""`, "", 1), "invalid identity XPath [", xsderrors.CodeSchemaIdentity, 3, 26},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			engine, err := xsd.Compile(xsd.Bytes(test.name+".xsd", []byte(test.source)))
			if engine != nil {
				t.Fatal("failed compilation returned an engine")
			}
			expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, test.code)
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic == nil {
				t.Fatalf("missing structured diagnostic: %v", err)
			}
			if diagnostic.Message() != test.message || diagnostic.Path() != test.name+".xsd" ||
				diagnostic.Line() != test.line || diagnostic.Column() != test.column || diagnostic.Cause() != nil {
				t.Fatalf("diagnostic = %v; want %q at %s.xsd:%d:%d with nil cause", diagnostic, test.message, test.name, test.line, test.column)
			}
		})
	}
}
