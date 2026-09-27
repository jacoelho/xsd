package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityMixedFieldSelectionsSurviveSessionReuse(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="row" maxOccurs="unbounded">
      <xs:complexType><xs:sequence>
        <xs:element name="v" type="xs:string" minOccurs="0"/>
        <xs:element name="w" type="xs:string" minOccurs="0"/>
      </xs:sequence>
      <xs:attribute name="a" type="xs:string"/>
      <xs:attribute name="b" type="xs:string"/>
      <xs:attribute name="c" type="xs:string"/>
      </xs:complexType>
    </xs:element></xs:sequence></xs:complexType>
    <xs:unique name="attributeBefore"><xs:selector xpath="row"/><xs:field xpath="@a"/></xs:unique>
    <xs:unique name="mixed"><xs:selector xpath="row"/><xs:field xpath="@b | w"/></xs:unique>
    <xs:unique name="element"><xs:selector xpath="row"/><xs:field xpath="v"/></xs:unique>
    <xs:unique name="attributeAfter"><xs:selector xpath="row"/><xs:field xpath="@c"/></xs:unique>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("mixed-fields.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const valid = `<root><row a="a1" b="b1" c="c1"><v>v1</v></row><row a="a2" c="c2"><v>v2</v><w>b2</w></row></root>`
	for _, test := range []struct {
		name string
		xml  string
	}{
		{name: "attribute before indexed fields", xml: `<root><row a="same"/><row a="same"/></root>`},
		{name: "attribute after indexed fields", xml: `<root><row c="same"/><row c="same"/></root>`},
		{name: "element field", xml: `<root><row><v>same</v></row><row><v>same</v></row></root>`},
		{name: "mixed field across rows", xml: `<root><row b="same"/><row><w>same</w></row></root>`},
		{name: "mixed field selects two nodes", xml: `<root><row b="one"><w>two</w></row></root>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			expectCategoryCode(t, session.Validate(strings.NewReader(test.xml)), xsderrors.CategoryValidation, xsderrors.CodeValidationIdentity)
			if err := session.Validate(strings.NewReader(valid)); err != nil {
				t.Fatalf("valid document after identity failure: %v", err)
			}
		})
	}
}
