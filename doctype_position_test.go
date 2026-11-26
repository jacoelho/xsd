package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestDOCTYPERejectedAtDeclarationStart(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:string"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("s.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name         string
		doc          string
		line, column int
	}{
		{"before the root", "\n<?p x?><!DOCTYPE r><r/>", 2, 8},
		{"after the root", "<r/><!DOCTYPE r>", 1, 5},
		{"inside content", "<r>\n  <!DOCTYPE r></r>", 2, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(test.doc))
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("error = %T %v, want structured error", err, err)
			}
			if diagnostic.Code() != xsderrors.CodeUnsupportedDTD {
				t.Fatalf("code = %v, want unsupported.dtd", diagnostic.Code())
			}
			if diagnostic.Line() != test.line || diagnostic.Column() != test.column {
				t.Errorf("location = %d:%d, want %d:%d", diagnostic.Line(), diagnostic.Column(), test.line, test.column)
			}
		})
	}
}
