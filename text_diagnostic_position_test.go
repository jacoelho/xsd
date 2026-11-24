package xsd_test

import (
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
