package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestLiveTypesBesideZeroCountElementTypesValidate(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("zero-count.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="B"><xs:simpleContent><xs:extension base="xs:string"><xs:attribute name="b" type="xs:int"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="hollow" minOccurs="0" maxOccurs="0"><xs:complexType/></xs:element>
    <xs:element name="restricted" minOccurs="0" maxOccurs="0"><xs:complexType><xs:simpleContent><xs:restriction base="B"><xs:maxLength value="3"/></xs:restriction></xs:simpleContent></xs:complexType></xs:element>
    <xs:element name="all" minOccurs="0" maxOccurs="0"><xs:complexType><xs:all><xs:element name="c"/><xs:element name="d"/></xs:all></xs:complexType></xs:element>
    <xs:element name="live" type="B"/>
    <xs:element name="empty"><xs:complexType/></xs:element>
    <xs:element name="pair"><xs:complexType><xs:all><xs:element name="c"/><xs:element name="d"/></xs:all></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, test := range []struct {
		name     string
		document string
		valid    bool
	}{
		{name: "live siblings", document: `<root><live b="1">long text</live><empty/><pair><d/><c/></pair></root>`, valid: true},
		{name: "shared base attribute type", document: `<root><live b="x">t</live><empty/><pair><c/><d/></pair></root>`},
		{name: "live empty content", document: `<root><live>t</live><empty>text</empty><pair><c/><d/></pair></root>`},
		{name: "live all group incomplete", document: `<root><live>t</live><empty/><pair><c/></pair></root>`},
		{name: "zero-count element present", document: `<root><hollow/><live>t</live><empty/><pair><c/><d/></pair></root>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(test.document))
			if test.valid && err != nil {
				t.Fatalf("Validate(%s) error = %v, want valid", test.document, err)
			}
			if test.valid {
				return
			}
			if diagnostic, ok := errors.AsType[*xsderrors.Error](err); !ok || diagnostic.Category() != xsderrors.CategoryValidation {
				t.Fatalf("Validate(%s) error = %v, want a validation diagnostic", test.document, err)
			}
		})
	}
}
