package runtime_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestQNameValueUsesInstanceNamespacesWithoutInterning(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="q" type="xs:QName"/>
</xs:schema>`)
	localsBefore := engineRuntime(t, engine).LocalNameCount()
	mustValidate(t, engine, `<q xmlns:p="urn:dynamic">p:notInSchema</q>`)
	if got := engineRuntime(t, engine).LocalNameCount(); got != localsBefore {
		t.Fatalf("QName validation interned instance local names: before=%d after=%d", localsBefore, got)
	}
	mustNotValidate(t, engine, `<q>p:notBound</q>`, xsderrors.CodeValidationFacet)
	mustNotValidate(t, engine, `<q>bad:name:shape</q>`, xsderrors.CodeValidationFacet)
}

func TestWildcardAllowsURIMatchesCompilePredicate(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t" xmlns="urn:t" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="e1"><xs:complexType><xs:sequence><xs:any namespace="##any" processContents="lax" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>
        <xs:element name="e2"><xs:complexType><xs:sequence><xs:any namespace="##other" processContents="lax" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>
        <xs:element name="e3"><xs:complexType><xs:sequence><xs:any namespace="##local" processContents="lax" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>
        <xs:element name="e4"><xs:complexType><xs:sequence><xs:any namespace="##targetNamespace" processContents="lax" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>
        <xs:element name="e5"><xs:complexType><xs:sequence><xs:any namespace="urn:a urn:b" processContents="lax" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	modes := make(map[runtime.WildcardMode]bool)
	for i := range rt.WildcardCount() {
		w, ok := rt.WildcardView(runtime.WildcardID(i))
		if !ok {
			t.Fatalf("missing wildcard %d", i)
		}
		modes[w.Mode()] = true
		for id := range rt.NamespaceCount() {
			uri := rt.Namespace(runtime.NamespaceID(id))
			if got, want := w.AllowsURI(uri), expectedWildcardURI(w.Mode(), uri); got != want {
				t.Errorf("wildcardAllowsURI(mode %d, %q) = %v, want %v", w.Mode(), uri, got, want)
			}
		}
		got := w.AllowsURI("urn:not-in-schema")
		want := expectedWildcardURI(w.Mode(), "urn:not-in-schema")
		if got != want {
			t.Errorf("wildcardAllowsURI(mode %d, uninterned URI) = %v, want %v", w.Mode(), got, want)
		}
	}
	for _, mode := range []runtime.WildcardMode{
		runtime.WildcardAny,
		runtime.WildcardOther,
		runtime.WildcardLocal,
		runtime.WildcardTargetNamespace,
		runtime.WildcardList,
	} {
		if !modes[mode] {
			t.Errorf("schema did not produce wildcard mode %d", mode)
		}
	}
}

func expectedWildcardURI(mode runtime.WildcardMode, uri string) bool {
	switch mode {
	case runtime.WildcardAny:
		return true
	case runtime.WildcardOther:
		return uri != "" && uri != "urn:t"
	case runtime.WildcardLocal:
		return uri == ""
	case runtime.WildcardTargetNamespace:
		return uri == "urn:t"
	case runtime.WildcardList:
		return uri == "urn:a" || uri == "urn:b"
	default:
		return false
	}
}
