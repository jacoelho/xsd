package tests_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/internal/format"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestFormatAndValidateReportNoRootAtSamePosition(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:anyType"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("s.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, doc, message string
		line               int
	}{
		{"comment and spaces", "<!-- c -->\n  ", "no root element", 2},
		{"processing instruction", "\n\n<?pi?>\n", "no root element", 4},
		{"unclosed element", "<r>\n  <a>\n", "</a>", 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			validateErr, ok := errors.AsType[*xsderrors.Error](engine.Validate(strings.NewReader(test.doc)))
			if !ok {
				t.Fatal("Validate: want structured error")
			}
			var out strings.Builder
			formatErr, ok := errors.AsType[*xsderrors.Error](format.XMLWithOptions(&out, test.doc, format.Options{}))
			if !ok {
				t.Fatal("format.XMLWithOptions: want structured error")
			}
			if validateErr.Line() != formatErr.Line() || validateErr.Column() != formatErr.Column() {
				t.Errorf("validate at %d:%d, format at %d:%d", validateErr.Line(), validateErr.Column(), formatErr.Line(), formatErr.Column())
			}
			if formatErr.Line() != test.line {
				t.Errorf("format line = %d, want %d", formatErr.Line(), test.line)
			}
			if !strings.Contains(formatErr.Error(), test.message) || strings.Contains(formatErr.Error(), "empty") {
				t.Errorf("format message = %q, want %q", formatErr.Error(), test.message)
			}
		})
	}
}

func TestValidateKeepsMismatchedEndTagMessageAndPosition(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:anyType"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("s.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](engine.Validate(strings.NewReader("<r>\n</s>")))
	if !ok {
		t.Fatal("want structured error")
	}
	if diagnostic.Code() != xsderrors.CodeValidationXML || diagnostic.Line() != 2 || diagnostic.Column() != 1 || diagnostic.Path() != "/r" {
		t.Errorf("diagnostic = %v at %d:%d path %q", diagnostic, diagnostic.Line(), diagnostic.Column(), diagnostic.Path())
	}
	if !strings.Contains(diagnostic.Error(), `end element "s" does not match start element "r"`) {
		t.Errorf("message = %q", diagnostic.Error())
	}
}

func TestValidateStatesEndOfDocumentFailureOnce(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:anyType"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("s.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, doc, want string }{
		{"empty document", "", "validation.xml at 1:1 /: instance document has no root element"},
		{"unclosed element", "<r>\n  <a>\n", "validation.xml at 3:1 /r/a: unclosed element"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(test.doc))
			if err == nil || err.Error() != test.want {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}
