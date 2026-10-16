package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func contextBorrowingSources() []xsd.SchemaSource {
	const common = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified" attributeFormDefault="unqualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element name="item" type="xs:string" minOccurs="0" maxOccurs="unbounded"/></xs:sequence>
      <xs:attribute name="id" type="xs:string" use="required"/>
      <xs:attribute name="ref" type="xs:string"/>
    </xs:complexType>
    <xs:keyref name="refs" refer="ids"><xs:selector xpath="."/><xs:field xpath="@ref"/></xs:keyref>
    <xs:key name="ids"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		if location != "common.xsd" {
			return xsd.SchemaSource{}, fmt.Errorf("unexpected schema location %q", location)
		}
		return xsd.Bytes("common.xsd", []byte(common)), nil
	})
	sources := make([]xsd.SchemaSource, 2)
	for i, namespace := range []string{"urn:a", "urn:b"} {
		root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + namespace + `" elementFormDefault="unqualified" attributeFormDefault="qualified"><xs:include schemaLocation="common.xsd"/></xs:schema>`
		sources[i] = xsd.Bytes(namespace+".xsd", []byte(root)).WithResolver(resolver)
	}
	return sources
}

func TestChameleonViewsPreserveIdentityAndSourceDefaults(t *testing.T) {
	t.Parallel()
	engine, err := xsd.Compile(contextBorrowingSources()...)
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, namespace := range []string{"urn:a", "urn:b"} {
		valid := `<t:root xmlns:t="` + namespace + `" id="x" ref="x"><t:item>v</t:item></t:root>`
		if err := session.Validate(strings.NewReader(valid)); err != nil {
			t.Fatalf("Validate(%q): %v", valid, err)
		}
		for _, invalid := range []struct {
			document string
			code     xsderrors.Code
			message  string
		}{
			{`<t:root xmlns:t="` + namespace + `" id="x" ref="missing"><t:item>v</t:item></t:root>`, xsderrors.CodeValidationIdentity, "keyref does not resolve"},
			{`<t:root xmlns:t="` + namespace + `" id="x" ref="x"><item>v</item></t:root>`, xsderrors.CodeValidationElement, "unexpected child element item"},
			{`<t:root xmlns:t="` + namespace + `" t:id="x" t:ref="x"/>`, xsderrors.CodeValidationAttribute, "attribute is not declared: id"},
		} {
			err := session.Validate(strings.NewReader(invalid.document))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, invalid.code)
			if !errorTreeContains(err, invalid.message) {
				t.Fatalf("Validate(%q): %v, want %q", invalid.document, err, invalid.message)
			}
			if err := session.Validate(strings.NewReader(valid)); err != nil {
				t.Fatalf("Validate(recovery): %v", err)
			}
		}
	}
}
