package validate

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
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

func TestResolveLexicalQNameParts(t *testing.T) {
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

			gotNS, gotLocal, gotOK := ResolveLexicalQNameParts(tt.lexical, lookup)
			if gotNS != tt.wantNS || gotLocal != tt.wantLocal || gotOK != tt.wantOK {
				t.Fatalf("ResolveLexicalQNameParts() = (%q, %q, %v), want (%q, %q, %v)",
					gotNS, gotLocal, gotOK, tt.wantNS, tt.wantLocal, tt.wantOK)
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

func TestRootStartMissingDeclarationIsRecoverable(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	got, err := RootStart(rt, nil, RootInput{
		Name:        xml.Name{Local: "root"},
		RuntimeName: runtime.RuntimeName{Local: "root"},
		Context:     StartContext{Line: 2, Column: 3, Path: "/"},
	})
	if err == nil {
		t.Fatal("RootStart() error is nil")
	}
	if !got.Skip || !got.Recover || got.Type != rt.AnyType() {
		t.Fatalf("RootStart() = %+v, want recoverable skip with anyType", got)
	}
	expectXSDCode(t, err, xsderrors.CodeValidationRoot)
}

func TestRootStartSchemaLocationHintIsUnsupported(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	got, err := RootStart(rt, nil, RootInput{
		Name:              xml.Name{Space: "urn:missing", Local: "root"},
		RuntimeName:       runtime.RuntimeName{NS: "urn:missing", Local: "root"},
		HasSchemaLocation: func(ns string) bool { return ns == "urn:missing" },
		Context:           StartContext{Line: 2, Column: 3, Path: "/"},
	})
	if err == nil {
		t.Fatal("RootStart() error is nil")
	}
	if got.Recover {
		t.Fatalf("RootStart() recover = true, want false")
	}
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
}

func TestResolveXSITypeSchemaHintUsesResolvedLocalName(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	ctx := StartContext{Line: 2, Column: 3, Path: "/root"}
	const want = "unsupported.xsi_schema_location at 2:3 /root: xsi:schemaLocation loading is not supported for type {urn:missing}Missing"
	for _, lexical := range []string{"p:Missing", "Missing"} {
		t.Run(lexical, func(t *testing.T) {
			t.Parallel()

			_, err := resolveXSIType(
				rt,
				lexical,
				func(value string) (string, string, bool) {
					if value != lexical {
						return "", "", false
					}
					return "urn:missing", "Missing", true
				},
				func(ns string) bool { return ns == "urn:missing" },
				ctx,
			)
			if err == nil || err.Error() != want {
				t.Fatalf("resolveXSIType(%q) error = %v, want %q", lexical, err, want)
			}
			expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
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
	rt, err := compile.Compile(compile.Options{}, []source.Source{
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

func xsiAttr(local, value string) stream.Attr {
	return stream.OwnedAttr(xml.Name{Space: vocab.XSINamespaceURI, Local: local}, value)
}

func startAttr(ns, local, value string) stream.Attr {
	return stream.OwnedAttr(xml.Name{Space: ns, Local: local}, value)
}

func startAttrs(attrs ...stream.Attr) []stream.Attr {
	return stream.OwnedAttrs(attrs...)
}

func expectXSDCode(t *testing.T, err error, code xsderrors.Code) {
	t.Helper()
	var x *xsderrors.Error
	if !errors.As(err, &x) {
		t.Fatalf("error = %v, want *xsderrors.Error", err)
	}
	if x.Code != code {
		t.Fatalf("error code = %s, want %s", x.Code, code)
	}
}

func compileRuntimeForTest(t *testing.T, schema string) *runtime.Schema {
	t.Helper()
	rt, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err != nil {
		t.Fatal(err)
	}
	return rt
}
