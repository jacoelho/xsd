package xsd

import "testing"

func TestQNameValueUsesInstanceNamespacesWithoutInterning(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="q" type="xs:QName"/>
</xs:schema>`)
	localsBefore := len(engine.rt.Names.locals)
	mustValidate(t, engine, `<q xmlns:p="urn:dynamic">p:notInSchema</q>`)
	if got := len(engine.rt.Names.locals); got != localsBefore {
		t.Fatalf("QName validation interned instance local names: before=%d after=%d", localsBefore, got)
	}
	mustNotValidate(t, engine, `<q>p:notBound</q>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<q>bad:name:shape</q>`, ErrValidationFacet)
}

func TestParseXSINil(t *testing.T) {
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
		value, ok := parseXSINil(tt.lexical)
		if value != tt.value || ok != tt.ok {
			t.Errorf("parseXSINil(%q) = (%v, %v), want (%v, %v)", tt.lexical, value, ok, tt.value, tt.ok)
		}
	}
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
	rt := engine.rt
	modes := make(map[wildcardMode]bool)
	for _, w := range rt.Wildcards {
		modes[w.Mode] = true
		for id := range rt.Names.namespaces {
			nsID := namespaceID(id)
			uri := rt.Names.Namespace(nsID)
			if got, want := rt.wildcardAllowsURI(w, uri), wildcardAllowsNamespace(w, nsID); got != want {
				t.Errorf("wildcardAllowsURI(mode %d, %q) = %v, want %v", w.Mode, uri, got, want)
			}
		}
		got := rt.wildcardAllowsURI(w, "urn:not-in-schema")
		want := w.Mode == wildAny || w.Mode == wildOther
		if got != want {
			t.Errorf("wildcardAllowsURI(mode %d, uninterned URI) = %v, want %v", w.Mode, got, want)
		}
	}
	for _, mode := range []wildcardMode{wildAny, wildOther, wildLocal, wildTargetNamespace, wildList} {
		if !modes[mode] {
			t.Errorf("schema did not produce wildcard mode %d", mode)
		}
	}
}
