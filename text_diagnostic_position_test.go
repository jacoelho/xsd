package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestTextDiagnosticLocatedAtFirstTextByte(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="empty"><xs:complexType><xs:attribute name="a"/></xs:complexType></xs:element></xs:sequence></xs:complexType></xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("text.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, content string
	}{
		{"line feed first", "\n\tElement Text\n  "},
		{"carriage return first", "\r\tElement Text\r  "},
		{"crlf first", "\r\n\tElement Text\r\n  "},
		{"text first", "Element Text\n"},
		{"reference first", "&#10;Element Text"},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := "<root>\n\t<empty a=\"v\">" + test.content + "</empty>\n</root>"
			assertStartDiagnostics(t, engine.Validate(strings.NewReader(doc)), []startDiagnostic{{xsderrors.CodeValidationText, "/root/empty", 2, 15}})
		})
	}
}

func TestElementOnlyTextDiagnosticLocatedAtFirstTextByte(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child" minOccurs="0"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("element-only.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range []string{"<root>\nx</root>", "<root>\rx</root>", "<root>\r\nx</root>"} {
		assertStartDiagnostics(t, engine.Validate(strings.NewReader(doc)), []startDiagnostic{{xsderrors.CodeValidationText, "/root", 1, 7}})
	}
}

func TestSchemaTextLimitDiagnosticLocatedAtFirstTextByte(t *testing.T) {
	for _, lineBreak := range []string{"\n", "\r", "\r\n"} {
		head := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>` + strings.Repeat("a", 50) + "<!--c-->"
		schema := head + lineBreak + strings.Repeat("b", 50) + `</xs:documentation></xs:annotation></xs:schema>`
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTokenBytes: 80}, xsd.Bytes("limit.xsd", []byte(schema)))
		leaves := xsderrors.Flatten(err)
		if len(leaves) != 1 {
			t.Fatalf("%q: diagnostics = %v, want one", lineBreak, leaves)
		}
		diagnostic, ok := errors.AsType[*xsderrors.Error](leaves[0])
		if !ok || diagnostic.Code() != xsderrors.CodeSchemaLimit || diagnostic.Line() != 1 || diagnostic.Column() != len(head)+1 {
			t.Fatalf("%q: error = %v, want %s at 1:%d", lineBreak, err, xsderrors.CodeSchemaLimit, len(head)+1)
		}
	}
}
