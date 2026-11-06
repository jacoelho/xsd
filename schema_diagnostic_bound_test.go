package xsd_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

// Schema diagnostics quote schema text, so hostile attribute values, names,
// and references must neither inflate a message nor reach terminals as raw
// control characters. This table is the contract proof;
// tests/schema_diagnostic_text_test.go is only an allowlist lint over the
// quoting call sites.
func TestSchemaDiagnosticsBoundHostileText(t *testing.T) {
	t.Parallel()

	const (
		maxMessage = 1024
		maxError   = 2048
		xsNS       = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`
	)
	// XML 1.0 cannot carry ESC, so the hostile value uses the control-like
	// characters it can: DEL, a C1 control, and tab, LF and CR by reference.
	// The leading controls are inside the displayed prefix; the long tail is
	// not.
	value := "\x7f\u0085&#9;&#10;&#13;" + strings.Repeat("x", 100_000)
	name := strings.Repeat("x", 100_000)

	wrap := func(body string) string {
		return `<xs:schema ` + xsNS + ` xmlns:t="urn:t">` + body + `</xs:schema>`
	}
	restrict := func(base, facets string) string {
		return wrap(`<xs:simpleType name="T"><xs:restriction base="` + base + `">` + facets + `</xs:restriction></xs:simpleType>`)
	}
	identity := func(inner string) string {
		return wrap(`<xs:element name="r"><xs:complexType><xs:sequence><xs:element name="c" type="xs:string"/></xs:sequence></xs:complexType>` + inner + `</xs:element>`)
	}

	tests := []struct {
		name   string
		schema string
	}{
		{"length facet", restrict("xs:string", `<xs:length value="`+value+`"/>`)},
		{"maxLength facet", restrict("xs:string", `<xs:maxLength value="`+value+`"/>`)},
		{"totalDigits facet", restrict("xs:decimal", `<xs:totalDigits value="`+value+`"/>`)},
		{"whiteSpace facet", restrict("xs:string", `<xs:whiteSpace value="`+value+`"/>`)},
		{"unsupported facet", restrict("xs:string", `<xs:`+name+` value="1"/>`)},
		{"enumeration", restrict("xs:int", `<xs:enumeration value="`+value+`"/>`)},
		{"element fixed", wrap(`<xs:element name="e" type="xs:int" fixed="` + value + `"/>`)},
		{"element default", wrap(`<xs:element name="e" type="xs:int" default="` + value + `"/>`)},
		{"attribute default", wrap(`<xs:attribute name="a" type="xs:int" default="` + value + `"/>`)},
		{"minOccurs", wrap(`<xs:element name="e"><xs:complexType><xs:sequence><xs:element name="c" minOccurs="` + value + `"/></xs:sequence></xs:complexType></xs:element>`)},
		{"maxOccurs", wrap(`<xs:element name="e"><xs:complexType><xs:sequence><xs:element name="c" maxOccurs="` + value + `"/></xs:sequence></xs:complexType></xs:element>`)},
		{"simpleType final", wrap(`<xs:simpleType name="T" final="` + value + `"><xs:restriction base="xs:string"/></xs:simpleType>`)},
		{"element block", wrap(`<xs:element name="e" block="` + value + `"/>`)},
		{"schema finalDefault", `<xs:schema ` + xsNS + ` finalDefault="` + value + `"/>`},
		{"wildcard namespace", wrap(`<xs:element name="e"><xs:complexType><xs:sequence><xs:any namespace="##` + name + `"/></xs:sequence></xs:complexType></xs:element>`)},
		{"identity selector", identity(`<xs:unique name="u"><xs:selector xpath="` + value + `"/><xs:field xpath="c"/></xs:unique>`)},
		{"identity field", identity(`<xs:unique name="u"><xs:selector xpath="c"/><xs:field xpath="` + value + `"/></xs:unique>`)},
		{"identity selector name", identity(`<xs:unique name="u"><xs:selector xpath="` + name + `:c"/><xs:field xpath="c"/></xs:unique>`)},
		{"keyref refer", identity(`<xs:keyref name="k" refer="` + name + `"><xs:selector xpath="c"/><xs:field xpath="c"/></xs:keyref>`)},
		{"duplicate identity", identity(`<xs:unique name="` + name + `"><xs:selector xpath="c"/><xs:field xpath="c"/></xs:unique><xs:unique name="` + name + `"><xs:selector xpath="c"/><xs:field xpath="c"/></xs:unique>`)},
		{"unbound prefix", wrap(`<xs:element name="e" type="` + name + `:x"/>`)},
		{"unknown type", wrap(`<xs:element name="e" type="t:` + name + `"/>`)},
		{"invalid QName", wrap(`<xs:element name="e" type="` + value + `"/>`)},
		{"duplicate type", wrap(`<xs:simpleType name="` + name + `"><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="` + name + `"><xs:restriction base="xs:string"/></xs:simpleType>`)},
		{"boolean attribute", wrap(`<xs:element name="e" abstract="` + value + `"/>`)},
		{"form attribute", wrap(`<xs:element name="e"><xs:complexType><xs:sequence><xs:element name="c" form="` + value + `"/></xs:sequence></xs:complexType></xs:element>`)},
		{"foreign element", wrap(`<xs:element name="e"><xs:complexType><xs:sequence/></xs:complexType></xs:element><` + name + ` xmlns="urn:foreign"/>`)},
		{"unknown schema child", wrap(`<xs:` + name + `/>`)},
		{"unknown attribute", wrap(`<xs:element name="e" ` + name + `="1"/>`)},
		{"include location", wrap(`<xs:include schemaLocation="` + value + `"/>`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			failing := xsd.ResolverFunc(func(base, location string) (xsd.SchemaSource, error) {
				return xsd.SchemaSource{}, errors.New("unavailable")
			})
			_, err := xsd.Compile(xsd.Bytes("s.xsd", []byte(tt.schema)).WithResolver(failing))
			if err == nil {
				t.Fatal("hostile schema compiled")
			}
			var diagnostic *xsderrors.Error
			if !errors.As(err, &diagnostic) {
				t.Fatalf("error %T is not a diagnostic: %v", err, err)
			}
			assertTerminalSafe(t, "Message", diagnostic.Message(), maxMessage)
			assertTerminalSafe(t, "Error", err.Error(), maxError)
		})
	}
}

func assertTerminalSafe(t *testing.T, what, text string, limit int) {
	t.Helper()
	if len(text) > limit {
		t.Errorf("%s is %d bytes, want at most %d: %.200q", what, len(text), limit, text)
	}
	if !utf8.ValidString(text) {
		t.Errorf("%s is not valid UTF-8", what)
	}
	for i := range len(text) {
		if c := text[i]; c < 0x20 || c == 0x7f {
			t.Errorf("%s contains raw control byte %#x: %.200q", what, c, text)
			return
		}
	}
	if strings.ContainsRune(text, '\u0085') {
		t.Errorf("%s contains raw C1 control U+0085: %.200q", what, text)
	}
}
