package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestLazyDocumentIdentityThroughDynamicAnyType(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="id" type="xs:anyType"/>
      <xs:element name="ref" type="xs:anyType"/>
    </xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("dynamic-identity.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	const namespaces = `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema"`
	valid := `<root ` + namespaces + `><id xsi:type="xs:ID">one</id><ref xsi:type="xs:IDREF">one</ref></root>`
	if validationErr := engine.Validate(strings.NewReader(valid)); validationErr != nil {
		t.Fatalf("Validate(valid dynamic identity) error = %v", validationErr)
	}
	missing := `<root ` + namespaces + `><id xsi:type="xs:ID">one</id><ref xsi:type="xs:IDREF">missing</ref></root>`
	err = engine.Validate(strings.NewReader(missing))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationType)
	if !strings.Contains(err.Error(), "IDREF does not resolve: missing") {
		t.Fatalf("Validate(missing dynamic IDREF) error = %v, want unresolved IDREF", err)
	}
}

func TestLazyDocumentIdentityThroughLaxWildcardDeclarations(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="id" type="xs:ID"/>
  <xs:element name="ref" type="xs:IDREF"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:any processContents="lax" maxOccurs="2"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("wildcard-identity.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if validationErr := engine.Validate(strings.NewReader(`<root><id>one</id><ref>one</ref></root>`)); validationErr != nil {
		t.Fatalf("Validate(valid wildcard identity) error = %v", validationErr)
	}
	err = engine.Validate(strings.NewReader(`<root><id>one</id><ref>missing</ref></root>`))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationType)
	if !strings.Contains(err.Error(), "IDREF does not resolve: missing") {
		t.Fatalf("Validate(missing wildcard IDREF) error = %v, want unresolved IDREF", err)
	}
}

func TestLazyDocumentIdentityThroughCompositeType(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="value"><xs:union memberTypes="xs:int xs:ID"/></xs:simpleType>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="item" type="value" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("composite-identity.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if validationErr := engine.Validate(strings.NewReader(`<root><item>7</item><item>+7</item><item>first</item><item>second</item></root>`)); validationErr != nil {
		t.Fatalf("Validate(valid composite identity) error = %v", validationErr)
	}
	err = engine.Validate(strings.NewReader(`<root><item>7</item><item>+7</item><item>same</item><item>same</item></root>`))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationType)
	if !strings.Contains(err.Error(), "duplicate ID same") {
		t.Fatalf("Validate(duplicate composite ID) error = %v, want duplicate ID", err)
	}
}
