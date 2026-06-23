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
	localsBefore := engineRuntime(t, engine).Names.LocalCount()
	mustValidate(t, engine, `<q xmlns:p="urn:dynamic">p:notInSchema</q>`)
	if got := engineRuntime(t, engine).Names.LocalCount(); got != localsBefore {
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
	for _, w := range rt.Wildcards {
		modes[w.Mode] = true
		for id := range rt.Names.NamespaceCount() {
			nsID := runtime.NamespaceID(id)
			uri := rt.Names.Namespace(nsID)
			if got, want := rt.WildcardAllowsURIForTest(w, uri), runtime.WildcardAllowsNamespace(w, nsID); got != want {
				t.Errorf("wildcardAllowsURI(mode %d, %q) = %v, want %v", w.Mode, uri, got, want)
			}
		}
		got := rt.WildcardAllowsURIForTest(w, "urn:not-in-schema")
		want := w.Mode == runtime.WildcardAny || w.Mode == runtime.WildcardOther
		if got != want {
			t.Errorf("wildcardAllowsURI(mode %d, uninterned URI) = %v, want %v", w.Mode, got, want)
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
