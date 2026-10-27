package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompileComplexExtensionBaseOccurrence(t *testing.T) {
	for _, compositor := range []string{"choice", "sequence"} {
		for _, occurrence := range []struct {
			name       string
			attributes string
			zero       bool
			two        bool
			three      bool
		}{
			{name: "unit"},
			{name: "optional", attributes: `minOccurs="0"`, zero: true},
			{name: "repeated", attributes: `maxOccurs="2"`, two: true},
			{name: "unbounded", attributes: `minOccurs="0" maxOccurs="unbounded"`, zero: true, two: true, three: true},
		} {
			t.Run(compositor+"/"+occurrence.name, func(t *testing.T) {
				source := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="FrameType">
    <xs:%s %s><xs:element name="a"/></xs:%s>
  </xs:complexType>
  <xs:complexType name="ModelType">
    <xs:complexContent><xs:extension base="FrameType">
      <xs:sequence><xs:element name="b"/></xs:sequence>
    </xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="ModelType"/>
</xs:schema>`, compositor, occurrence.attributes, compositor)
				engine, err := xsd.Compile(xsd.Bytes("extension.xsd", []byte(source)))
				if err != nil {
					t.Fatalf("Compile() error = %v", err)
				}
				for _, instance := range []struct {
					xml   string
					valid bool
				}{
					{`<root><a/><b/></root>`, true},
					{`<root><b/></root>`, occurrence.zero},
					{`<root><a/><a/><b/></root>`, occurrence.two},
					{`<root><a/><a/><a/><b/></root>`, occurrence.three},
					{`<root><b/><a/></root>`, false},
					{`<root><a/></root>`, false},
				} {
					err := engine.Validate(strings.NewReader(instance.xml))
					if instance.valid && err != nil {
						t.Errorf("Validate(%s) error = %v", instance.xml, err)
					}
					if !instance.valid && !xsderrors.IsConclusiveValidation(err) {
						t.Errorf("Validate(%s) error = %v, want conclusive validation failure", instance.xml, err)
					}
				}
			})
		}
	}
}

func TestCompileComplexExtensionRepeatsWholeBaseSequence(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("extension.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence maxOccurs="2"><xs:element name="a"/><xs:element name="c"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base">
      <xs:sequence><xs:element name="b"/></xs:sequence>
    </xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="Derived"/>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, xml := range []string{`<root><a/><c/><b/></root>`, `<root><a/><c/><a/><c/><b/></root>`} {
		if err := engine.Validate(strings.NewReader(xml)); err != nil {
			t.Errorf("Validate(%s) error = %v", xml, err)
		}
	}
	const invalid = `<root><a/><a/><c/><c/><b/></root>`
	if err := engine.Validate(strings.NewReader(invalid)); !xsderrors.IsConclusiveValidation(err) {
		t.Errorf("Validate(%s) error = %v, want conclusive validation failure", invalid, err)
	}
}
