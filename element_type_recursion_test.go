package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

// A named type derived from a base can be reached through element
// declarations while that base is still being compiled. That is recursion
// through elements, not a derivation cycle, and must compile.
func TestCompileExtensionReachedThroughBaseContent(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">
  <xs:complexType name="Base"><xs:sequence><xs:element name="prompt" type="Prompt" minOccurs="0"/></xs:sequence></xs:complexType>
  <xs:complexType name="Prompt" mixed="true"><xs:choice minOccurs="0" maxOccurs="unbounded"><xs:element ref="object"/></xs:choice></xs:complexType>
  <xs:complexType name="Drawing"><xs:complexContent><xs:extension base="Base"><xs:sequence><xs:element ref="object"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
  <xs:complexType name="Object" mixed="true"><xs:choice minOccurs="0" maxOccurs="unbounded"><xs:element name="drawing" type="Drawing"/></xs:choice></xs:complexType>
  <xs:complexType name="DerivedDrawing"><xs:complexContent><xs:extension base="Drawing"><xs:attribute name="a" type="xs:string"/></xs:extension></xs:complexContent></xs:complexType>
  <xs:element name="object" type="Object"/>
  <xs:element name="root" type="DerivedDrawing"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	valid := `<root xmlns="urn:t" a="x"><prompt>p<object><drawing><object/></drawing></object></prompt><object/></root>`
	if err := engine.Validate(strings.NewReader(valid)); err != nil {
		t.Fatalf("valid document: %v", err)
	}
	for _, doc := range []string{
		`<root xmlns="urn:t"><prompt/></root>`,
		`<root xmlns="urn:t"><object><drawing/></object></root>`,
	} {
		if err := engine.Validate(strings.NewReader(doc)); err == nil {
			t.Fatalf("%s: want validation error", doc)
		}
	}
}

func TestCompileRejectsDerivationCycleThroughElementReference(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A"><xs:sequence><xs:element name="b" type="B"/></xs:sequence></xs:complexType>
  <xs:complexType name="B"><xs:complexContent><xs:extension base="C"/></xs:complexContent></xs:complexType>
  <xs:complexType name="C"><xs:complexContent><xs:extension base="B"/></xs:complexContent></xs:complexType>
  <xs:element name="root" type="A"/>
</xs:schema>`
	if _, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema))); err == nil {
		t.Fatal("want cyclic derivation error")
	}
}

// An anonymous type reached the same way is deferred by the same mechanism.
func TestCompileAnonymousExtensionReachedThroughBaseContent(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">
  <xs:complexType name="Base"><xs:sequence><xs:element name="prompt" type="Prompt" minOccurs="0"/></xs:sequence></xs:complexType>
  <xs:complexType name="Prompt" mixed="true"><xs:choice minOccurs="0" maxOccurs="unbounded"><xs:element ref="object"/></xs:choice></xs:complexType>
  <xs:element name="object">
    <xs:complexType mixed="true"><xs:choice minOccurs="0" maxOccurs="unbounded">
      <xs:element name="drawing"><xs:complexType><xs:complexContent><xs:extension base="Base"><xs:attribute name="a" type="xs:string" use="required"/></xs:extension></xs:complexContent></xs:complexType></xs:element>
    </xs:choice></xs:complexType>
  </xs:element>
  <xs:element name="root" type="Base"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := engine.Validate(strings.NewReader(`<root xmlns="urn:t"><prompt><object><drawing a="x"><prompt/></drawing></object></prompt></root>`)); err != nil {
		t.Fatalf("valid document: %v", err)
	}
	if err := engine.Validate(strings.NewReader(`<root xmlns="urn:t"><prompt><object><drawing/></object></prompt></root>`)); err == nil {
		t.Fatal("missing required attribute: want validation error")
	}
}
