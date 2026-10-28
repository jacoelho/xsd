package validate

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	xsdValue "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestResolveRuntimeName(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="known"/></xs:schema>`)
	known, ok := rt.LookupQName("", "known")
	if !ok {
		t.Fatal("compiled runtime is missing known name")
	}
	got := ResolveRuntimeName(rt, xml.Name{Local: "known"})
	if !got.Known || got.Name != known || got.NS != "" || got.Local != "known" {
		t.Fatalf("ResolveRuntimeName() known = %+v", got)
	}
	got = ResolveRuntimeName(rt, xml.Name{Space: "urn:missing", Local: "unknown"})
	if got.Known || got.NS != "urn:missing" || got.Local != "unknown" {
		t.Fatalf("ResolveRuntimeName() unknown = %+v", got)
	}
}

func TestResolveLexicalQName(t *testing.T) {
	t.Parallel()

	lookup := func(prefix string) (string, bool) {
		switch prefix {
		case "":
			return "urn:default", true
		case "p":
			return "urn:p", true
		default:
			return "", false
		}
	}
	tests := []struct {
		name      string
		lexical   string
		wantNS    string
		wantLocal string
		wantOK    bool
	}{
		{name: "unprefixed", lexical: "local", wantNS: "urn:default", wantLocal: "local", wantOK: true},
		{name: "prefixed", lexical: "p:local", wantNS: "urn:p", wantLocal: "local", wantOK: true},
		{name: "xml whitespace collapsed", lexical: "\t p:local \n", wantNS: "urn:p", wantLocal: "local", wantOK: true},
		{name: "empty prefix", lexical: ":local"},
		{name: "empty local", lexical: "p:"},
		{name: "multiple colons", lexical: "p:a:b"},
		{name: "invalid prefix", lexical: "1p:local"},
		{name: "invalid local", lexical: "p:1local"},
		{name: "unknown prefix", lexical: "missing:local"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, gotOK := ResolveLexicalQName(tt.lexical, lookup)
			if got.Namespace != tt.wantNS || got.Local != tt.wantLocal || gotOK != tt.wantOK {
				t.Fatalf("ResolveLexicalQName() = (%q, %q, %v), want (%q, %q, %v)",
					got.Namespace, got.Local, gotOK, tt.wantNS, tt.wantLocal, tt.wantOK)
			}
		})
	}
}

func TestParseXSINil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		lexical string
		value   bool
		ok      bool
	}{
		{"true", true, true},
		{"false", false, true},
		{"1", true, true},
		{"0", false, true},
		{" true ", true, true},
		{"\ttrue\n", true, true},
		{"yes", false, false},
		{"TRUE", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		value, ok := ParseXSINil(tt.lexical)
		if value != tt.value || ok != tt.ok {
			t.Errorf("ParseXSINil(%q) = (%v, %v), want (%v, %v)", tt.lexical, value, ok, tt.value, tt.ok)
		}
	}
}

func TestXSIStartAttributeFlagsType(t *testing.T) {
	t.Parallel()

	if xsiStartAttributeFlagsFor(nil).Type {
		t.Fatal("xsiStartAttributeFlagsFor(nil).Type = true, want false")
	}
	if xsiStartAttributeFlagsFor(startAttrs(startAttr("urn:xsi-like", vocab.XSIAttrType, ""))).Type {
		t.Fatal("xsiStartAttributeFlagsFor(other namespace).Type = true, want false")
	}
	if xsiStartAttributeFlagsFor(startAttrs(xsiAttr(vocab.XSIAttrNil, "true"))).Type {
		t.Fatal("xsiStartAttributeFlagsFor(xsi:nil).Type = true, want false")
	}
	if !xsiStartAttributeFlagsFor(startAttrs(startAttr("", "id", ""), xsiAttr(vocab.XSIAttrType, "p:D"))).Type {
		t.Fatal("xsiStartAttributeFlagsFor(xsi:type).Type = false, want true")
	}
}

func TestResolveXSITypeReportsUnknownType(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	for _, lexical := range []string{"p:Missing", "Missing"} {
		t.Run(lexical, func(t *testing.T) {
			t.Parallel()

			_, err := resolveXSIType(
				rt,
				lexical,
				func(value string) (xsdValue.ExpandedName, bool) {
					if value != lexical {
						return xsdValue.ExpandedName{}, false
					}
					return xsdValue.ExpandedName{Namespace: "urn:missing", Local: "Missing"}, true
				},
			)
			want := "validation.type: xsi:type \"" + lexical + "\" resolves to \"{urn:missing}Missing\", but no global type declaration is available"
			if err == nil || err.Error() != want {
				t.Fatalf("resolveXSIType(%q) error = %v, want %q", lexical, err, want)
			}
			expectXSDCode(t, err, xsderrors.CodeValidationType)
		})
	}
}

func TestSessionStartOwnsXSITypeAndNilPolicy(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    targetNamespace="urn:t" xmlns:t="urn:t" elementFormDefault="qualified">
  <xs:complexType name="Base"/>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="t:Base"/></xs:complexContent>
  </xs:complexType>
  <xs:complexType name="TypeBlockedBase" block="extension"/>
  <xs:complexType name="TypeBlockedDerived">
    <xs:complexContent><xs:extension base="t:TypeBlockedBase"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="allowed" type="t:Base" nillable="true" block="substitution"/>
  <xs:element name="blocked" type="t:Base" block="extension"/>
  <xs:element name="typeBlocked" type="t:TypeBlockedBase"/>
</xs:schema>`
	rt, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(schema)),
	})

	if err != nil {
		t.Fatal(err)
	}
	s, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	const namespaces = `xmlns:t="urn:t" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`
	err = s.Validate(strings.NewReader(`<t:allowed ` + namespaces + ` xsi:type="t:Derived" xsi:nil="true"/>`))
	if err != nil {
		t.Fatalf("valid xsi:type/xsi:nil: %v", err)
	}
	err = s.Validate(strings.NewReader(`<t:blocked ` + namespaces + ` xsi:type="t:Derived"/>`))
	expectXSDCode(t, err, xsderrors.CodeValidationType)
	err = s.Validate(strings.NewReader(`<t:typeBlocked ` + namespaces + ` xsi:type="t:TypeBlockedDerived"/>`))
	expectXSDCode(t, err, xsderrors.CodeValidationType)
}

func xsiAttr(local, value string) xmlstream.Attr {
	return xmlstream.Attr{Name: xml.Name{Space: vocab.XSINamespaceURI, Local: local}, Value: value}
}

func startAttr(ns, local, value string) xmlstream.Attr {
	return xmlstream.Attr{Name: xml.Name{Space: ns, Local: local}, Value: value}
}

func startAttrs(attrs ...xmlstream.Attr) []xmlstream.Attr {
	return attrs
}

func expectXSDCode(t *testing.T, err error, code xsderrors.Code) {
	t.Helper()
	var x *xsderrors.Error
	if !errors.As(err, &x) {
		t.Fatalf("error = %v, want *xsderrors.Error", err)
	}
	if x.Code() != code {
		t.Fatalf("error code = %s, want %s", x.Code(), code)
	}
}

func compileRuntimeForTest(t *testing.T, schema string) *xsdSchema.Schema {
	t.Helper()
	rt, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

// Structures 3.3.4 Element Locally Valid (Element) 4.3 and Type Derivation OK
// (Complex) 1 block xsi:type separately; the message names the declaration
// when both block the same method.
func TestXSITypeBlockNamesBlockingComponent(t *testing.T) {
	t.Parallel()

	const (
		byDeclaration = `xsi:type "Ext" uses extension, which the element declaration blocks`
		byType        = `xsi:type "Ext" uses extension, which declared type "Base" blocks`
	)
	for _, test := range []struct {
		name, schemaAttr, elementAttr, typeAttr, want string
	}{
		{name: "neither", want: ""},
		{name: "element", elementAttr: `block="extension"`, want: byDeclaration},
		{name: "type", typeAttr: `block="extension"`, want: byType},
		{name: "blockDefault on type", schemaAttr: `blockDefault="extension"`, elementAttr: `block=""`, want: byType},
		{name: "blockDefault on element", schemaAttr: `blockDefault="extension"`, typeAttr: `block=""`, want: byDeclaration},
		{name: "both", elementAttr: `block="extension"`, typeAttr: `block="extension"`, want: byDeclaration},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" `+test.schemaAttr+`>
  <xs:complexType name="Base" `+test.typeAttr+`><xs:sequence/></xs:complexType>
  <xs:complexType name="Ext"><xs:complexContent><xs:extension base="Base"/></xs:complexContent></xs:complexType>
  <xs:element name="root" type="Base" `+test.elementAttr+`/>
</xs:schema>`)
			err := NewSessionPool(rt).Validate(strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Ext"/>`), Options{})
			if test.want == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want valid", err)
				}
				return
			}
			var x *xsderrors.Error
			if !errors.As(err, &x) || x.Code() != xsderrors.CodeValidationType || x.Message() != test.want {
				t.Fatalf("Validate() = %v, want %s %q", err, xsderrors.CodeValidationType, test.want)
			}
		})
	}
}

// Simple-value diagnostics name the nearest built-in type (xs:int, not the
// xs:integer lexical family), report integer-family syntax as integer, and
// keep QName syntax errors distinct from unbound prefixes.
func TestSimpleValueDiagnosticNamesNearestBuiltin(t *testing.T) {
	t.Parallel()

	const qnameList = `<xs:simpleType><xs:list itemType="xs:QName"/></xs:simpleType>`

	for _, test := range []struct {
		name, typeAttr, simpleType, input, want string
	}{
		{name: "int control", typeAttr: `type="xs:int"`, input: "1"},
		{name: "anonymous int", simpleType: `<xs:simpleType><xs:restriction base="xs:int"/></xs:simpleType>`, input: "x", want: `value "x" is not valid for xs:int: invalid integer`},
		{name: "anonymous ID", simpleType: `<xs:simpleType><xs:restriction base="xs:ID"/></xs:simpleType>`, input: "1x", want: `value "1x" is not valid for xs:ID: invalid NCName`},
		{name: "int", typeAttr: `type="xs:int"`, input: "x", want: `value "x" is not valid for xs:int: invalid integer`},
		{name: "byte", typeAttr: `type="xs:byte"`, input: "x", want: `value "x" is not valid for xs:byte: invalid integer`},
		{name: "unsignedShort", typeAttr: `type="xs:unsignedShort"`, input: "x", want: `value "x" is not valid for xs:unsignedShort: invalid integer`},
		{name: "nonNegativeInteger", typeAttr: `type="xs:nonNegativeInteger"`, input: "x", want: `value "x" is not valid for xs:nonNegativeInteger: invalid integer`},
		{name: "long", typeAttr: `type="xs:long"`, input: "x", want: `value "x" is not valid for xs:long: invalid integer`},
		{name: "integer", typeAttr: `type="xs:integer"`, input: "x", want: `value "x" is not valid for xs:integer: invalid integer`},
		{name: "decimal", typeAttr: `type="xs:decimal"`, input: "x", want: `value "x" is not valid for xs:decimal: invalid decimal`},
		{name: "QName list syntax", simpleType: qnameList, input: "a 1x", want: `value "a 1x" is not a valid list of xs:QName: invalid QName syntax; expected a local name or prefix:local name`},
		{name: "QName list unbound prefix", simpleType: qnameList, input: "a p:b", want: `value "a p:b" is not a valid list of xs:QName: unresolved QName`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			element := `<xs:element name="root" ` + test.typeAttr + `/>`
			if test.simpleType != "" {
				element = `<xs:element name="root">` + test.simpleType + `</xs:element>`
			}
			rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+element+`</xs:schema>`)
			err := NewSessionPool(rt).Validate(strings.NewReader(`<root>`+test.input+`</root>`), Options{})
			if test.want == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want valid", err)
				}
				return
			}
			var x *xsderrors.Error
			if !errors.As(err, &x) || x.Message() != "invalid simple content: "+test.want {
				t.Fatalf("Validate() = %v, want %q", err, test.want)
			}
		})
	}
}
