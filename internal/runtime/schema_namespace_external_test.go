package runtime_test

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestQNameValueUsesInstanceNamespacesWithoutInterning(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="q" type="xs:QName"/>
</xs:schema>`)
	rt := engineRuntime(t, engine)
	if _, ok := rt.LookupQName("", "notInSchema"); ok {
		t.Fatal("test local name is already interned")
	}
	mustValidate(t, engine, `<q xmlns:p="urn:dynamic">p:notInSchema</q>`)
	if _, ok := rt.LookupQName("", "notInSchema"); ok {
		t.Fatal("QName validation interned an instance local name")
	}
	mustNotValidate(t, engine, `<q>p:notBound</q>`, xsderrors.CodeValidationFacet)
	mustNotValidate(t, engine, `<q>bad:name:shape</q>`, xsderrors.CodeValidationFacet)
}

func TestWildcardNamespacePredicates(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t" xmlns="urn:t" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
		<xs:element name="e1" minOccurs="0"><xs:complexType><xs:sequence><xs:any namespace="##any" processContents="lax"/></xs:sequence></xs:complexType></xs:element>
		<xs:element name="e2" minOccurs="0"><xs:complexType><xs:sequence><xs:any namespace="##other" processContents="lax"/></xs:sequence></xs:complexType></xs:element>
		<xs:element name="e3" minOccurs="0"><xs:complexType><xs:sequence><xs:any namespace="##local" processContents="lax"/></xs:sequence></xs:complexType></xs:element>
		<xs:element name="e4" minOccurs="0"><xs:complexType><xs:sequence><xs:any namespace="##targetNamespace" processContents="lax"/></xs:sequence></xs:complexType></xs:element>
		<xs:element name="e5" minOccurs="0"><xs:complexType><xs:sequence><xs:any namespace="urn:a urn:b" processContents="lax"/></xs:sequence></xs:complexType></xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	for _, tt := range []struct {
		name  string
		doc   string
		valid bool
	}{
		{name: "any target", doc: `<root xmlns="urn:t"><e1><x/></e1></root>`, valid: true},
		{name: "any local", doc: `<root xmlns="urn:t"><e1><x xmlns=""/></e1></root>`, valid: true},
		{name: "other unknown", doc: `<root xmlns="urn:t"><e2><x xmlns="urn:not-in-schema"/></e2></root>`, valid: true},
		{name: "other target", doc: `<root xmlns="urn:t"><e2><x/></e2></root>`},
		{name: "other local", doc: `<root xmlns="urn:t"><e2><x xmlns=""/></e2></root>`},
		{name: "local", doc: `<root xmlns="urn:t"><e3><x xmlns=""/></e3></root>`, valid: true},
		{name: "local rejects target", doc: `<root xmlns="urn:t"><e3><x/></e3></root>`},
		{name: "target", doc: `<root xmlns="urn:t"><e4><x/></e4></root>`, valid: true},
		{name: "target rejects local", doc: `<root xmlns="urn:t"><e4><x xmlns=""/></e4></root>`},
		{name: "list first", doc: `<root xmlns="urn:t"><e5><x xmlns="urn:a"/></e5></root>`, valid: true},
		{name: "list second", doc: `<root xmlns="urn:t"><e5><x xmlns="urn:b"/></e5></root>`, valid: true},
		{name: "list rejects unknown", doc: `<root xmlns="urn:t"><e5><x xmlns="urn:not-in-schema"/></e5></root>`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				mustValidate(t, engine, tt.doc)
				return
			}
			mustNotValidate(t, engine, tt.doc, xsderrors.CodeValidationElement)
		})
	}
}
