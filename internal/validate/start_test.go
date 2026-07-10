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

type startRuntimeStub struct {
	root       map[runtime.QName]runtime.ElementID
	elements   map[runtime.ElementID]runtime.ElementStartInfo
	types      map[runtime.QName]runtime.TypeID
	names      map[expandedName]runtime.QName
	namespaces map[runtime.NamespaceID]string
	anyType    runtime.TypeID
}

type expandedName struct {
	ns    string
	local string
}

func (s startRuntimeStub) AnyType() runtime.TypeID {
	return s.anyType
}

func (s startRuntimeStub) RootElement(name runtime.RuntimeName) (runtime.ElementID, runtime.ElementStartInfo, bool) {
	if !name.Known {
		return runtime.NoElement, runtime.ElementStartInfo{}, false
	}
	id, ok := s.root[name.Name]
	if !ok {
		return runtime.NoElement, runtime.ElementStartInfo{}, false
	}
	info, ok := s.elements[id]
	return id, info, ok
}

func (s startRuntimeStub) Type(name runtime.QName) (runtime.TypeID, bool) {
	typ, ok := s.types[name]
	return typ, ok
}

func (s startRuntimeStub) LookupQName(ns, local string) (runtime.QName, bool) {
	q, ok := s.names[expandedName{ns: ns, local: local}]
	return q, ok
}

func (s startRuntimeStub) Namespace(id runtime.NamespaceID) string {
	return s.namespaces[id]
}

func TestResolveRuntimeName(t *testing.T) {
	t.Parallel()

	known := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	rt := startRuntimeStub{
		names: map[expandedName]runtime.QName{
			{local: "known"}: known,
		},
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

	rt := startRuntimeStub{anyType: runtime.ComplexRef(1)}
	got, err := RootStart(rt, nil, RootInput{
		Name:        xml.Name{Local: "root"},
		RuntimeName: runtime.RuntimeName{Local: "root"},
		Context:     StartContext{Line: 2, Column: 3, Path: "/"},
	})
	if err == nil {
		t.Fatal("RootStart() error is nil")
	}
	if !got.Skip || !got.Recover || got.Type != rt.anyType {
		t.Fatalf("RootStart() = %+v, want recoverable skip with anyType", got)
	}
	expectXSDCode(t, err, xsderrors.CodeValidationRoot)
}

func TestRootStartSchemaLocationHintIsUnsupported(t *testing.T) {
	t.Parallel()

	rt := startRuntimeStub{anyType: runtime.ComplexRef(1)}
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

func TestSessionStartOwnsXSITypeAndNilPolicy(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    targetNamespace="urn:t" xmlns:t="urn:t" elementFormDefault="qualified">
  <xs:complexType name="Base"/>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="t:Base"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="allowed" type="t:Base" nillable="true" block="substitution"/>
  <xs:element name="blocked" type="t:Base" block="extension"/>
</xs:schema>`
	rt, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(schema)),
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(rt, Options{})
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
}

func xsiAttr(local, value string) stream.Attr {
	return stream.Attr{Name: xml.Name{Space: vocab.XSINamespaceURI, Local: local}, Value: value}
}

func startAttr(ns, local, value string) stream.Attr {
	return stream.Attr{Name: xml.Name{Space: ns, Local: local}, Value: value}
}

func startAttrs(attrs ...stream.Attr) []stream.Attr {
	return attrs
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
